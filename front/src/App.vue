<template>
    <p>mabi damage meter</p>
    <template v-for="v, i in filteredEntityList" v-bind:key="i">
        <p>{{ v.Name }} {{ v.TotalDamage.toFixed(0) }}</p>
        <CProgress :height="32">
            <CProgressBar v-for="damageByAttacker, attackerId in v.TotalDamageByAttacker" v-bind:key="attackerId" @click="showDetailDamageList(v.Id, attackerId)" :value="100 * damageByAttacker / v.TotalDamage"
                variant="striped" animated :style="`background-color: ${getNameColor(entityMap[attackerId]?.Name)}`">
                {{ entityMap[attackerId]?.Name || attackerId }} {{ damageByAttacker.toFixed(0) }}
            </CProgressBar>
        </CProgress>

        <CModal :visible="!!detailDialogData" @close="() => { detailDialogData = undefined }">
            <CModalHeader>
                <CModalTitle>{{ detailDialogData?.attackerName }} -> {{ detailDialogData?.targetName }}</CModalTitle>
            </CModalHeader>
            <CModalBody>
                <CContainer v-for="v, i in detailDialogData?.Damages" v-bind:key="i">
                    <p>
                        <img width="32" height="32"
                    :src='`http://localhost:${__api_port}/res/skillimage/${region}/${v.SkillId}/${v.SkillId}.png`' />
                        {{ skillNameMap[v.SkillId] }} {{ v.Damage.toFixed(0) }} {{ v.IsCritical ? '크리티컬' : '' }}
                    </p>
                </CContainer>

            </CModalBody>
            <CModalFooter>
                <CButton color="secondary" @click="() => { detailDialogData = undefined }">
                    Close
                </CButton>
            </CModalFooter>
        </CModal>
    </template>
</template>

<script lang="ts">
import { defineComponent, onMounted, inject, ref, computed } from "vue";

import { CProgressBar, CProgress, CModal, CModalHeader, CModalTitle, CModalBody, CModalFooter, CButton, CContainer } from '@coreui/bootstrap-vue';

export default defineComponent({
    name: "App",
    components: {
        CProgressBar,
        CProgress,
        CModal,
        CModalHeader,
        CModalTitle,
        CModalBody,
        CModalFooter,
        CButton,
        CContainer,
    },
    setup() {
        const isLoading = inject('isLoading');
        const region = inject('region');
        const lang = inject('lang');
        const regionList = inject('regionList');
        const db = inject('db');

        const socket = ref<WebSocket>();

        const openSocket = () => {
            const s = socket.value = new WebSocket(`ws://localhost:${__api_port}/ws`)

            socket.value.onopen = () => {
                console.log("socket open");
            };
            socket.value.onmessage = e => {
                const event = JSON.parse(e.data) as eventBase;
                switch (event?.EventId) {
                    case 1:
                        const entityAppear = event as eventEntityAppear;
                        onEventEntityAppear(entityAppear);
                        break;

                    case 3:
                        const damage = event as eventDamage;
                        onEventDamage(damage);
                        break;
                }
            };

            socket.value.onerror = e => {
                console.log('socket error', e);
                s.close();

                // setTimeout(openSocket, 0);
            };
            socket.value.onclose = e => {
                console.log('socket close', e);
                s.close();

                setTimeout(openSocket, 0);
            };
        };

        
        const onEventEntityAppear = (event: eventEntityAppear) => {
            const origName = event.Name;
            let name = origName;
            if (name[0] == '_') {
                // npc
                return;
            }

            if (name[0] >= '0' && name[0] <= '9') {
                // mob, field mob은 pool이 돈다
                name = `${raceNameMap.value[event.RaceId]}(${event.Id.substring(event.Id.length - 4)})`
            }

            entityMap.value[event.Id] = {
                Id: event.Id,
                Name: name,
                OrigName: origName,
                TotalDamage: 0,
                Damages: [],
                TotalDamageByAttacker: {},
            };
        };

        const onEventDamage = (event: eventDamage) => {
            const id = event.TargetId;
            const entity = entityMap.value[id];
            if (!entity) {
                // ?
                return;
            }

            entityMap.value[id].TotalDamage += event.Damage;
            entityMap.value[id].Damages.push(event);

            if (!entity.TotalDamageByAttacker[event.Id]) {
                entity.TotalDamageByAttacker[event.Id] = 0;
            }

            entity.TotalDamageByAttacker[event.Id] += event.Damage;
        };

        const showDetailDamageList = (targetId: string, attackerId: string) => {
            const entity = entityMap.value[targetId];
            if (!entity) {
                return;
            }

            detailDialogData.value = {
                targetName: entity.Name,
                attackerName: entityMap.value[attackerId]?.Name || attackerId,
                Damages: entity.Damages.filter(v => v.Id == attackerId),
            };
        }

        const entityMap = ref<Record<string, entityInfo>>({});
        const filteredEntityList = computed<entityInfo[]>(() => {
            return Object.values(entityMap.value).filter(v => v.TotalDamage > 0);
        });

        const detailDialogData = ref<{targetName: string; attackerName: string; Damages: entityDamage[]}>();

        const raceNameMap = ref<Record<number, string>>({});
        const skillNameMap = ref<Record<number, string>>({});

        onMounted(async () => {
            regionList.value = ['kr', 'krt', 'cn', 'jp', 'tw', 'us'];

            await db.value.tryOpen();
            {
                const list = await db.value.getSortedListData('RaceList');

                for (const v of list) {
                    raceNameMap.value[v.Id] = db.value.getCurLangString(v.Name);
                }
            }
            {
                const list = await db.value.getSortedListData('SkillList');

                for (const v of list) {
                    skillNameMap.value[v.Id] = db.value.getCurLangString(v.Name);
                }
            }

            openSocket();
        });

        return {
            __api_port,
            region,

            entityMap,
            filteredEntityList,
            detailDialogData,

            skillNameMap, 

            showDetailDamageList,
            getNameColor,
        }
    }
});

function getNameColor(name: string): string {
    // https://mabinoger.com/color_sim.htm
    const colCalc = (i: number) => (i * 101) % 97 + 159;

    // if (name.length < 3) {
    //     return '#808080';
    // }

    const ccolor = [0, 0, 0];
    // R = (ASCII Char 1,4,7,10
    // G = (ASCII Char 2,5,8,11
    // B = (ASCII Char 3,6,9,12
    //                          * 101) mod 97) + 159
    for (var i = 0; i < name.length; i++) {
        ccolor[i % 3] += name.charCodeAt(i);
    }
    ccolor[0] = colCalc(ccolor[0]);
    ccolor[1] = colCalc(ccolor[1]);
    ccolor[2] = colCalc(ccolor[2]);

    return '#' + ccolor.map(v => v.toString(16).padStart(2, '0')).join('');
}

type eventId = number;

type eventBase = {
    EventId: eventId;
}

type eventEntityAppear = eventBase & {
    EventId: 1;
    Id: string;
    Name: string;
    RaceId: number;
}

type eventDamage = eventBase & {
    EventId: 3;
    Id: string;
    TargetId: string;
    SkillId: number;
    Damage: number;
    IsCritical: boolean;
}

type entityInfo = {
    Id: string;
    Name: string;
    OrigName: string;
    TotalDamage: number;
    Damages: entityDamage[];
    TotalDamageByAttacker: Record<string, number>;
}

type entityDamage = {
    Id: string;
    TargetId: string;
    SkillId: number;
    Damage: number;
    IsCritical: boolean;
}

</script>
