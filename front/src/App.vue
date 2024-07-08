<template>
    <p>mabi damage meter</p>
    <v-expansion-panels multiple v-for="v, k in entityGroupMap" v-bind:key="k">
        <template v-if="v.TotalDamage > 0">
            <v-expansion-panel>
                <v-expansion-panel-title>
                    <v-sheet>
                        {{ v.Name }} {{ v.TotalDamage.toFixed(0) }}
                    </v-sheet>

                </v-expansion-panel-title>
                <v-expansion-panel-text class="pa-3">
                    <template v-for="entity, entityk in v.EntityMap" v-bind:key="entityk">
                        <template v-if="entity.TotalDamage > 0">
                            <v-sheet>
                                {{ entity.Name }} {{ entity.TotalDamage.toFixed(0) }} {{ entity.FinisherId ? `Killed by
                                ${entityMap[entity.FinisherId]?.Name || entity.FinisherId}` : '' }}
                                <template v-for="cond in entity.ConditionMap" v-bind:key="cond.CCId">
                                    <img width="16" height="16"
                                        @mouseover="e => setCondTooltip(e.target! as HTMLElement, cond)"
                                        @mouseleave="e => setCondTooltip(e.target! as HTMLElement, undefined)"
                                        @click="e => setCondTooltip(e.target! as HTMLElement, cond)"
                                        :src='`http://localhost:${__api_port}/res/characterconditionimage/${region}/${cond.CCId}/${cond.CCId}.png`'>
                                    </img>
                                </template>
                            </v-sheet>

                            <v-sheet width="100%" :height="32" color="grey-lighten-3"
                                class="d-flex my-4 rounded-xl overflow-hidden">
                                <template v-for="damageByAttacker, attackerId in entity.TotalDamageByAttacker"
                                    v-bind:key="attackerId">
                                    <v-sheet @click="showEntityDetailDamageList(entity.Id, attackerId)"
                                        :color="getMabiNameColor(entityMap[attackerId]?.Name)" height="100%"
                                        :width="`${Math.round(100 * damageByAttacker / entity.TotalDamage).toFixed(0)}%`"
                                        class="text-center align-center">
                                        {{ entityMap[attackerId]?.Name || attackerId }} {{ damageByAttacker.toFixed(0)
                                        }}
                                    </v-sheet>
                                </template>
                            </v-sheet>
                        </template>
                    </template>
                </v-expansion-panel-text>
            </v-expansion-panel>
            <v-sheet width="100%" :height="32" color="grey-lighten-3" class="d-flex mb-4 rounded-xl overflow-hidden">
                <template v-for="damageByAttacker, attackerId in v.TotalDamageByAttacker" v-bind:key="attackerId">
                    <v-sheet @click.stop="showEntityGroupDetailDamageList(v.Id, attackerId)"
                        :color="getMabiNameColor(entityMap[attackerId]?.Name)" height="100%"
                        :width="`${Math.round(100 * damageByAttacker / v.TotalDamage).toFixed(0)}%`"
                        class="text-center align-center">
                        {{ entityMap[attackerId]?.Name || attackerId }} {{ damageByAttacker.toFixed(0) }}
                    </v-sheet>
                </template>
            </v-sheet>
        </template>

    </v-expansion-panels>

    <v-dialog v-model="detailDialog" min-width="60vw">
        <v-card>
            <v-card-text>
                <p>{{ detailDialogData?.attackerName }} -> {{ detailDialogData?.targetName }}</p>

                <v-sheet v-for="v, i in detailDialogData?.Damages" v-bind:key="i">
                    <p>
                        <template v-for="cond in v.Conditions" v-bind:key="cond.CCId">
                            <img width="16" height="16" @mouseover="e => setCondTooltip(e.target! as HTMLElement, cond)"
                                @mouseleave="e => setCondTooltip(e.target! as HTMLElement, undefined)"
                                @click="e => setCondTooltip(e.target! as HTMLElement, cond)"
                                :src='`http://localhost:${__api_port}/res/characterconditionimage/${region}/${cond.CCId}/${cond.CCId}.png`'>
                            </img>
                        </template>
                        ->
                        <template v-for="cond in v.TargetConditions" v-bind:key="cond.CCId">
                            <img width="16" height="16" @mouseover="e => setCondTooltip(e.target! as HTMLElement, cond)"
                                @mouseleave="e => setCondTooltip(e.target! as HTMLElement, undefined)"
                                @click="e => setCondTooltip(e.target! as HTMLElement, cond)"
                                :src='`http://localhost:${__api_port}/res/characterconditionimage/${region}/${cond.CCId}/${cond.CCId}.png`'>
                            </img>
                        </template>
                    </p>
                    <p>
                        <img width="32" height="32"
                            :src='`http://localhost:${__api_port}/res/skillimage/${region}/${v.SkillId}/${v.SkillId}.png`' />
                        {{ skillNameMap[v.SkillId] }} {{ v.Damage.toFixed(0) }} {{ v.IsCritical ? '크리티컬' : '' }}
                    </p>
                </v-sheet>
            </v-card-text>
            <v-card-actions>
                <v-btn color="primary" block @click="detailDialog = false">Close</v-btn>
            </v-card-actions>
        </v-card>
    </v-dialog>
    <v-tooltip v-if="condTooltip" v-model="condTooltipValue" :activator="condTooltipParent">
        {{ condNameMap[condTooltip.CCId] }}
    </v-tooltip>
</template>

<script lang="ts">
import { defineComponent, onMounted, inject, ref, computed } from "vue";
import { VTooltip } from 'vuetify/components';

import * as protocols from '@/protocols';
import { getMabiNameColor } from '@/util';

export default defineComponent({
    name: "App",
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
                const event = JSON.parse(e.data) as protocols.eventBase;
                console.log(event);

                switch (event?.EventId) {
                    case protocols.eventIdEntityAppear:
                        const entityAppear = event as protocols.eventEntityAppear;
                        onEventEntityAppear(entityAppear);
                        break;

                    case protocols.eventIdDamage:
                        const damage = event as protocols.eventDamage;
                        onEventDamage(damage);
                        break;

                    case protocols.eventIdCharacterConditionEnable:
                        const conditionEnable = event as protocols.eventCharacterConditionEnable;
                        onEventCharacterConditionEnable(conditionEnable);
                        break;

                    case protocols.eventIdCharacterConditionDisable:
                        const conditionDisable = event as protocols.eventCharacterConditionDisable;
                        onEventCharacterConditionDisable(conditionDisable);
                        break;

                    case protocols.eventIdFinish:
                        const finish = event as protocols.eventFinish;
                        onEventFinish(finish);
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


        const onEventEntityAppear = (event: protocols.eventEntityAppear) => {
            const origName = event.Name;
            let name = origName;
            const isPc = pcRaceSet.has(event.RaceId);

            if (!isPc) {
                // mob, field mob은 pool이 돈다
                name = `${raceNameMap.value[event.RaceId]}(${event.Id.substring(event.Id.length - 4)})`
            }

            const prevEntity = entityMap.value[event.Id];
            const entity = entityMap.value[event.Id] = {
                Id: event.Id,
                Name: name,
                OrigName: origName,
                TotalDamage: 0,
                Damages: [],
                TotalDamageByAttacker: {},
                ConditionMap: {},
                FinisherId: '',
                Group: undefined! as entityGroup,
            };

            const groupId = pcRaceSet.has(event.RaceId) ? event.Id : `${event.RaceId}`;

            const group = entityGroupMap.value[groupId] ??= {
                Id: groupId,
                Name: isPc ? name : raceNameMap.value[event.RaceId],
                EntityMap: {},
                TotalDamage: 0,
                Damages: [],
                TotalDamageByAttacker: {},
            };

            if (prevEntity) {
                group.TotalDamage -= prevEntity.TotalDamage;
                group.Damages = group.Damages.filter(v => v.Id != prevEntity.Id);
                for (const attackerId in prevEntity.TotalDamageByAttacker) {
                    group.TotalDamageByAttacker[attackerId] -= prevEntity.TotalDamageByAttacker[attackerId];
                }
            }

            group.EntityMap[event.Id] = entity;
            entity.Group = group;
        };

        const onEventDamage = (event: protocols.eventDamage) => {
            const id = event.TargetId;
            const entity = entityMap.value[id];
            if (!entity) {
                // ?
                return;
            }

            const attackerEntity = entityMap.value[event.Id];
            const damageData: entityDamage = {
                ...event,
                Conditions: attackerEntity ? Object.values(attackerEntity.ConditionMap) : [],
                TargetConditions: Object.values(entity.ConditionMap),
            }

            entityMap.value[id].TotalDamage += event.Damage;
            entityMap.value[id].Damages.push(damageData);

            if (!entity.TotalDamageByAttacker[event.Id]) {
                entity.TotalDamageByAttacker[event.Id] = 0;
            }

            entity.TotalDamageByAttacker[event.Id] += event.Damage;

            entity.Group.TotalDamage += event.Damage;
            entity.Group.Damages.push(damageData);
            if (!entity.Group.TotalDamageByAttacker[event.Id]) {
                entity.Group.TotalDamageByAttacker[event.Id] = 0;
            }

            entity.Group.TotalDamageByAttacker[event.Id] += event.Damage;
        };

        const onEventCharacterConditionEnable = (event: protocols.eventCharacterConditionEnable) => {
            const id = event.Id;
            const entity = entityMap.value[id];
            if (!entity) {
                // ?
                return;
            }

            entity.ConditionMap[event.CCId] = {
                Id: id,
                CCId: event.CCId,
                DisableAt: event.DisableAt,
                AttackerId: event.AttackerId,
            };
        };

        const onEventCharacterConditionDisable = (event: protocols.eventCharacterConditionDisable) => {
            const id = event.Id;
            const entity = entityMap.value[id];
            if (!entity) {
                // ?
                return;
            }

            delete entity.ConditionMap[event.CCId];
        };

        const onEventFinish = (event: protocols.eventFinish) => {
            if (!event.AttackerId) {
                return;
            }

            const entity = entityMap.value[event.Id];
            if (!entity) {
                // ?
                return;
            }

            entity.FinisherId = event.AttackerId;
            console.log(entity.Name, 'killed by', entityMap.value[event.AttackerId]?.Name || event.AttackerId);
        };

        const showEntityDetailDamageList = (targetId: string, attackerId: string) => {
            const entity = entityMap.value[targetId];
            if (!entity) {
                return;
            }

            detailDialog.value = true;
            detailDialogData.value = {
                targetName: entity.Name,
                attackerName: entityMap.value[attackerId]?.Name || attackerId,
                Damages: entity.Damages.filter(v => v.Id == attackerId),
            };
        }

        const showEntityGroupDetailDamageList = (targetId: string, attackerId: string) => {
            const group = entityGroupMap.value[targetId];
            if (!group) {
                return;
            }

            detailDialog.value = true;
            detailDialogData.value = {
                targetName: group.Name,
                attackerName: entityMap.value[attackerId]?.Name || attackerId,
                Damages: group.Damages.filter(v => v.Id == attackerId),
            };
        }

        const entityGroupMap = ref<Record<string, entityGroup>>({});
        const entityMap = ref<Record<string, entityInfo>>({});

        const detailDialog = ref(false);
        const detailDialogData = ref<{ targetName: string; attackerName: string; Damages: entityDamage[] }>();

        const raceNameMap = ref<Record<number, string>>({});
        const skillNameMap = ref<Record<number, string>>({});
        const condNameMap = ref<Record<number, string>>({});

        const condTooltipParent = ref<HTMLElement>();
        const condTooltipValue = ref(false);
        const condTooltip = ref<entityCondition>();

        const setCondTooltip = (el: HTMLElement, cond?: entityCondition) => {
            condTooltip.value = cond;
            condTooltipParent.value = el;
            condTooltipValue.value = !!cond;
        }

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
            {
                const list = await db.value.getSortedListData('CharCondList');

                for (const v of list) {
                    condNameMap.value[v.Id] = db.value.getCurLangString(v.Name);
                }
            }

            openSocket();
        });

        return {
            __api_port,
            region,

            entityMap,
            entityGroupMap,

            detailDialog,
            detailDialogData,

            skillNameMap,
            condNameMap,

            condTooltip,
            condTooltipParent,
            condTooltipValue,
            setCondTooltip,

            showEntityDetailDamageList,
            showEntityGroupDetailDamageList,
            getMabiNameColor,
        }
    }
});

type entityGroup = {
    Id: string;
    Name: string;
    EntityMap: Record<string, entityInfo>;
    TotalDamage: number;
    Damages: entityDamage[];
    TotalDamageByAttacker: Record<string, number>;
}

type entityInfo = {
    Id: string;
    Name: string;
    OrigName: string;
    TotalDamage: number;
    Damages: entityDamage[];
    TotalDamageByAttacker: Record<string, number>;
    ConditionMap: Record<number, entityCondition>;
    FinisherId: string;
    Group: entityGroup;
}

type entityDamage = {
    Id: string;
    TargetId: string;
    SkillId: number;
    Damage: number;
    IsCritical: boolean;
    Conditions: entityCondition[];
    TargetConditions: entityCondition[];
}

type entityCondition = {
    Id: string;
    CCId: number;
    DisableAt: number;
    AttackerId: string;
}

const pcRaceSet = new Set<number>([8001, 8002, 9001, 9002, 10001, 10002]);

</script>
