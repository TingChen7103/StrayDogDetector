<template>
    <v-sheet width="100vw" class="d-flex flex-wrap pl-1 pr-1">
        <v-sheet width="100svw" class="d-flex">
            <span style="text-wrap-mode: nowrap;">dilmatulgi, api
                <span v-if="socketConnected"><v-icon icon="mdi-check" color="success" />connected</span>
                <span v-else><v-icon icon="mdi-close" color="error" />disconnected</span>
            </span>
            <span>

            </span>
            <v-divider />
            <v-btn @click="download" :loading="isLoading" color="primary" size="small"
                prepend-icon="mdi-download" class="ml-1">Download</v-btn>
            <v-btn @click="clearData" :loading="isLoading" color="primary" size="small"
                prepend-icon="mdi-close" class="ml-1 mr-4">Clear</v-btn></v-sheet>
    </v-sheet>

    <v-tabs v-model="tab">
        <v-tab value="takeDamage">Take Damage</v-tab>
        <v-tab value="applyDamageByEntity">Apply Damage (By Entity)</v-tab>
        <!-- @TODO -->
        <!-- <v-tab value="applyDamageBySkill">Apply Damage (By Skill)</v-tab> -->
    </v-tabs>

    <v-tabs-window v-model="tab">
        <v-tabs-window-item value="takeDamage">
            <take-damage />
        </v-tabs-window-item>

        <v-tabs-window-item value="applyDamageByEntity">
            <apply-damage-by-entity />
        </v-tabs-window-item>
    </v-tabs-window>
</template>

<script lang="ts">
import { defineComponent, onMounted, inject, ref, reactive } from "vue";

import { SocketClient } from '@/socketClient';

import TakeDamageComponent from '@/components/takeDamage.vue';
import ApplyDamageByEntityComponent from '@/components/applyDamageByEntity.vue';

export default defineComponent({
    name: "App",
    components: {
        TakeDamage: TakeDamageComponent,
        ApplyDamageByEntity: ApplyDamageByEntityComponent,
    },
    setup() {
        const isLoading = inject('isLoading');
        const region = inject('region');
        // const lang = inject('lang');
        const regionList = inject('regionList');
        const db = inject('db');
        const raceNameMap = inject('raceNameMap');
        const skillNameMap = inject('skillNameMap');
        const condNameMap = inject('condNameMap');
        const actorManager = inject('actorManager');

        const socketConnected = ref(false);
        const socket = new SocketClient(`/ws`);
        socket.onConnect = isConnected => socketConnected.value = isConnected;
        socket.onEvent = (event) => actorManager.value.onEvent(event);

        const clearData = () => {
            actorManager.value.clear();
        }

        const download = () => {
            window.open('/api/packet_log', '_blank');
        }

        const tab = ref('');

        onMounted(async () => {
            regionList.value = ['kr', 'krt', 'cn', 'jp', 'tw', 'us'];

            await db.value.tryOpen();
            {
                const list = await db.value.getSortedListData('RaceList');

                for (const v of list) {
                    raceNameMap.value[v.Id] = `${db.value.getCurLangString(v.Name)} ${v.Id}`;
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
                    condNameMap.value[v.Id] = `${db.value.getCurLangString(v.Name)} ${v.Id}`;
                }
            }

            Object.assign(actorManager.value.entityMap, reactive({}));
            Object.assign(actorManager.value.groupMap, reactive({}));

            socket.connect();
        });

        return {
            isLoading,
            region,

            socketConnected,
            clearData,
            download,

            tab,
        }
    }
});

</script>
