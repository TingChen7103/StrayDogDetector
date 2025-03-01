<template>
    <v-expansion-panels multiple v-for="v in pcEntities" v-bind:key="v.id">
        <v-expansion-panel>
            <v-expansion-panel-title>
                <v-sheet>
                    {{ v.name }} {{ v.id }} {{ raceNameMap[v.raceId] }}
                </v-sheet>
            </v-expansion-panel-title>
            <v-expansion-panel-text class="pa-3">
                <v-sheet width="100%" class="mb-2">
                    <template v-for="cond in Object.values(v.conditionMap).sort((a, b) => a.CCId - b.CCId)"
                        v-bind:key="cond.CCId">
                        <img width="16" height="16" @mouseover="e => setCondTooltip(e.target! as HTMLElement, cond)"
                            @mouseleave="e => setCondTooltip(e.target! as HTMLElement, undefined)"
                            @click="e => setCondTooltip(e.target! as HTMLElement, cond)"
                            :src='`/res/characterconditionimage/${region}/${cond.CCId}/${cond.CCId}.png`' />
                    </template>
                </v-sheet>

                <v-sheet width="100%"
                    v-for="item in Object.values(v.equipItemMap).sort((a, b) => a.PocketType - b.PocketType)"
                    v-bind:key="item.PocketType" class="d-flex mb-2">
                    <v-sheet width="48px" height="96px"
                        :style='`background: url("/res/invimage/${region}/${item.ItemId}/${item.ItemId}.png") no-repeat; background-position: center;`' />
                    <v-sheet>
                        {{ itemNameMap[item.ItemId] }} {{ item.PocketType }}
                    </v-sheet>
                </v-sheet>
            </v-expansion-panel-text>
        </v-expansion-panel>
    </v-expansion-panels>

    <v-tooltip v-if="condTooltip" v-model="condTooltipValue" :activator="condTooltipParent">
        {{ condNameMap[condTooltip.CCId] }}
    </v-tooltip>
</template>

<script lang="ts">
import { defineComponent, inject, ref, computed, onMounted } from "vue";

import { getMabiNameColor } from '@/util';
import { EntityCondition } from '@/eventActor';

export default defineComponent({
    setup() {
        const isLoading = inject('isLoading');
        const region = inject('region');
        const raceNameMap = inject('raceNameMap');
        const condNameMap = inject('condNameMap');
        const itemNameMap = inject('itemNameMap');
        const actorManager = inject('actorManager');

        const pcEntities = computed(() =>
            Object.values(actorManager.value.entityMap).filter(v => v.isPC).sort((a, b) => a.name.localeCompare(b.name)));

        const condTooltipParent = ref<HTMLElement>();
        const condTooltipValue = ref(false);
        const condTooltip = ref<EntityCondition>();

        const setCondTooltip = (el: HTMLElement, cond?: EntityCondition) => {
            condTooltip.value = cond;
            condTooltipParent.value = el;
            condTooltipValue.value = !!cond;
        }

        onMounted(() => {
            console.log(pcEntities);
        });

        return {
            isLoading,
            region,

            pcEntities,
            raceNameMap,
            condNameMap,
            itemNameMap,

            condTooltip,
            condTooltipParent,
            condTooltipValue,
            setCondTooltip,

            getMabiNameColor,
        }
    }
});

</script>