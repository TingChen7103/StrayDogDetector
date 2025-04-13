<template>
    <v-sheet>
        <div ref="chartDom" style="width: 100svw; height: 300px;"></div>
    </v-sheet>
    <v-select v-model="targetId" :items="targetIdList"
        :item-title="vv => `${vv[0] ? prettyEntityName(entityMap[vv[0]]?.actor) : 'all'} ${vv[1]?.toFixed(0)}`"
        :item-value="vv => vv[0]" class="ma-2" variant="outlined" density="compact" hide-details>
    </v-select>

    <v-expansion-panels multiple v-for="v in pcEntities" v-bind:key="v.actor.id">
        <template v-if="v.totalDamage > 0">
            <v-expansion-panel>
                <v-expansion-panel-title>
                    <v-sheet>
                        {{ prettyEntityName(v.actor) }} {{ v.totalDamage.toFixed(0) }} {{ (100 * v.totalDamage /
                            allApplyDamage).toFixed(1) }}% dps {{ v.damages.length < 2 ? 0 : Math.round(v.totalDamage /
                            (v.damages[v.damages.length - 1].At - v.damages[0].At)) }} </v-sheet>

                </v-expansion-panel-title>
                <v-expansion-panel-text class="pa-3">
                    <v-sheet
                        v-for="[skillId, damageBySkill] in Object.entries(v.groupedTotalDamages).sort(([, av], [_, bv]) => bv - av)"
                        v-bind:key="skillId" class="d-flex">
                        <v-sheet width="32" class="mr-2">
                            <img width="32" height="32" :src='`/res/skillimage/${region}/${skillId}/${skillId}.png`' />
                        </v-sheet>
                        <v-sheet width="100%" class="mb-4">
                            <!-- 스킬 이미지 -->
                            <!-- 이름 -->
                            <v-sheet width="100%"
                                @click.stop="showEntityDetailDamageList(v.actor.id, targetId, +skillId)">
                                {{ skillNameMap[+skillId] || `unknownSkill:${skillId}` }}
                                damage: {{ damageBySkill.toFixed(0) }}
                                count: {{ v.groupedDamages[+skillId].length }}
                                avgDamage: {{ (damageBySkill / v.groupedDamages[+skillId].length).toFixed(0) }}
                                minDamage: {{ v.groupedMinDamages[+skillId].toFixed(0) }}
                                maxDamage: {{ v.groupedMaxDamages[+skillId].toFixed(0) }}
                                {{ (100 * damageBySkill / v.totalDamage).toFixed(1) }}%
                            </v-sheet>

                            <!-- 막대 -->
                            <v-sheet width="100%" height="16">
                                <v-sheet @click.stop="showEntityDetailDamageList(v.actor.id, '', +skillId)"
                                    :color="getMabiNameColor(skillNameMap[+skillId] || `unknownSkill:${skillId}`)"
                                    height="100%"
                                    :width="`${Math.round(100 * damageBySkill / v.totalDamage).toFixed(0)}%`"
                                    class="rounded-xl">
                                </v-sheet>
                            </v-sheet>
                        </v-sheet>
                    </v-sheet>
                </v-expansion-panel-text>
            </v-expansion-panel>
            <!-- 막대 -->
            <v-sheet width="100%" height="16">
                <v-sheet :color="getMabiNameColor(prettyEntityName(v.actor)!)" height="100%"
                    :width="`${Math.round(100 * v.totalDamage / allApplyDamage).toFixed(0)}%`" class="rounded-xl">
                </v-sheet>
            </v-sheet>
        </template>

    </v-expansion-panels>

    <v-dialog v-model="detailDialog" min-width="60vw" height="90svh">
        <v-card>
            <v-card-text class="pa-0">
                <v-sheet width="100%" class="d-flex pa-2 mb-2">
                    {{ detailDialogData?.attackerName }} {{ detailDialogData?.skillName }}
                </v-sheet>

                <v-virtual-scroll :items="detailDialogData?.damages"
                    style="min-height: 300px; height: calc(90svh - 200px)" item-height="80">
                    <template v-slot:default="{ item }">
                        <v-card min-height="80">
                            <v-sheet class="d-flex">
                                <v-sheet width="32" class="mr-2">
                                    <img width="32" height="32"
                                        :src='`/res/skillimage/${region}/${item.SkillId}/${item.SkillId}.png`' />
                                </v-sheet>

                                <v-sheet>
                                    <p>
                                        <template v-for="cond in item.Conditions" v-bind:key="cond.CCId">
                                            <img width="16" height="16"
                                                @mouseover="e => setCondTooltip(e.target! as HTMLElement, cond)"
                                                @mouseleave="e => setCondTooltip(e.target! as HTMLElement, undefined)"
                                                @click="e => setCondTooltip(e.target! as HTMLElement, cond)"
                                                :src='`/res/characterconditionimage/${region}/${cond.CCId}/${cond.CCId}.png`' />
                                        </template>
                                        ->
                                        <template v-for="cond in item.TargetConditions" v-bind:key="cond.CCId">
                                            <img width="16" height="16"
                                                @mouseover="e => setCondTooltip(e.target! as HTMLElement, cond)"
                                                @mouseleave="e => setCondTooltip(e.target! as HTMLElement, undefined)"
                                                @click="e => setCondTooltip(e.target! as HTMLElement, cond)"
                                                :src='`/res/characterconditionimage/${region}/${cond.CCId}/${cond.CCId}.png`' />
                                        </template>
                                    </p>
                                    <p>
                                        {{ prettyEntityName(entityMap[item.TargetId]?.actor) }}
                                        {{ item.Damage.toFixed(0) }} {{ item.IsCritical ? '크리티컬' : '' }}
                                    </p>
                                </v-sheet>
                            </v-sheet>
                        </v-card>
                    </template>
                </v-virtual-scroll>
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
import { defineComponent, inject, ref, computed, onUnmounted, watch, onMounted } from "vue";
import highcharts, { Options, SeriesAreaOptions } from 'highcharts';

import { getMabiNameColor } from '@/util';
import { EntityDamage, EntityCondition, ActorManager, BaseActor, GroupActor, EntityActor } from '@/eventActor';
import { DamageCollectorBase, DualGroupedDamageCollector, GroupedDamageCollector } from '@/actionCollector';

export default defineComponent({
    setup() {
        const isLoading = inject('isLoading');
        const region = inject('region');
        const raceNameMap = inject('raceNameMap');
        const skillNameMap = inject('skillNameMap');
        const condNameMap = inject('condNameMap');
        const actorManager = inject('actorManager');
        const dcManager = inject('dcManager');

        const damageCollectorMap: Record<string, DamageCollectorBase> = {};
        const chartDom = ref<HTMLElement>(undefined!);
        let chart: Highcharts.Chart = undefined!;

        const chartData = computed(() => getChartSeriesData(pcEntities.value.filter(v => v.totalDamage > 10000)));

        onUnmounted(() => {
            for (const v of Object.values(damageCollectorMap)) {
                dcManager.value.removeDamageCollector(v);
            }
        });

        const getDC = (attackerId: string) => {
            const key = `${attackerId}`;
            if (damageCollectorMap[key]) {
                return damageCollectorMap[key] as DualGroupedDamageCollector;
            }

            const dc = dcManager.value.getDualGroupedDamageCollector(v => v.Id == attackerId, v => v.TargetId, v => `${v.SkillId}`);
            damageCollectorMap[key] = dc;

            return dc;
        }

        const getTargetDC = () => {
            const key = `target`;
            if (damageCollectorMap[key]) {
                return damageCollectorMap[key] as GroupedDamageCollector;
            }

            const dc = dcManager.value.getGroupedDamageCollector(() => true, v => v.TargetId);
            damageCollectorMap[key] = dc;

            return dc;
        }
        const targetDC = ref(getTargetDC());

        const showEntityDetailDamageList = (attackerId: string, targetId: string, skillId: number) => {
            const entity = entityMap.value[attackerId];
            if (!entity) {
                return;
            }

            const damages = targetId ?
                entity.dc.dualGroupedDamages[targetId][skillId] :
                entity.dc.grouped2Damages[skillId];

            detailDialog.value = true;
            detailDialogData.value = {
                attackerName: prettyEntityName(entity.actor)!,
                skillName: skillNameMap.value[skillId] || `unknownSkill:${skillId}`,
                damages: damages,
            };
        }

        const entityMap = computed(() => {
            const m: Record<string, { actor: EntityActor, dc: DualGroupedDamageCollector }> = {};

            for (const k in actorManager.value.entityMap) {
                const v = actorManager.value.entityMap[k];

                m[k] = {
                    actor: v,
                    dc: getDC(k),
                }
            }

            return m;
        });

        const entityMapWithTargetData = computed(() => {
            const m: Record<string, EntityExtended> = {};

            for (const k in entityMap.value) {
                const v = entityMap.value[k];

                const totalDamage = targetId.value ? v.dc.groupedTotalDamages[targetId.value] || 0 : v.dc.totalDamage;
                const damages = targetId.value ? v.dc.groupedDamages[targetId.value] || [] : v.dc.damages;
                const groupedTotalDamages = targetId.value ? v.dc.dualGroupedTotalDamages[targetId.value] : v.dc.grouped2TotalDamages;
                const groupedDamages = targetId.value ? v.dc.dualGroupedDamages[targetId.value] : v.dc.grouped2Damages;
                const groupedMinDamages = targetId.value ? v.dc.dualGroupedMinDamages[targetId.value] : v.dc.grouped2MinDamages;
                const groupedMaxDamages = targetId.value ? v.dc.dualGroupedMaxDamages[targetId.value] : v.dc.grouped2MaxDamages;

                m[k] = {
                    ...v,
                    totalDamage,
                    damages,
                    groupedTotalDamages,
                    groupedDamages,
                    groupedMinDamages,
                    groupedMaxDamages,
                }
            }

            return m;
        })

        const pcEntities = computed(() =>
            Object.values(entityMapWithTargetData.value).filter(v => v.actor.isPC).sort((a, b) => b.totalDamage - a.totalDamage));

        const targetId = ref('');
        const targetIdList = computed(() => {
            const list = Object.entries(targetDC.value.groupedTotalDamages)
                .sort(([, av], [, bv]) => bv - av);

            list.unshift(['', targetDC.value.totalDamage]);

            return list;
        });

        watch(targetIdList, () => {
            if (!targetId.value) {
                return;
            }

            if (targetIdList.value.find(v => v[0] == targetId.value)) {
                return;
            }

            targetId.value = '';
        });


        onMounted(() => {
            let debounced1 = 0;
            watch(entityMap, () => {
                if (debounced1) {
                    return;
                }

                setTimeout(() => {
                    debounced1 = 0;
                    if (chart) {
                        chart.destroy();
                    }
                    chart = highcharts.chart(chartDom.value, { ...chartOpt, series: chartData.value });
                }, 100);
            });

            let debounced2: number = 0;
            watch(chartData, () => {
                if (debounced2) {
                    return;
                }

                debounced2 = setTimeout(() => {
                    debounced2 = 0;
                    chart?.update({ series: chartData.value });
                }, 100);
            });
        })

        const allApplyDamage = computed(() =>
            pcEntities.value.reduce((acc, v) => acc + v.totalDamage, 0));

        const detailDialog = ref(false);
        const detailDialogData = ref<{ attackerName: string; skillName: string; damages: EntityDamage[] }>();

        const condTooltipParent = ref<HTMLElement>();
        const condTooltipValue = ref(false);
        const condTooltip = ref<EntityCondition>();

        const setCondTooltip = (el: HTMLElement, cond?: EntityCondition) => {
            condTooltip.value = cond;
            condTooltipParent.value = el;
            condTooltipValue.value = !!cond;
        }

        const prettyEntityName = (entity?: BaseActor) => {
            if (!entity) {
                return undefined;
            }

            if (ActorManager.pcRaceSet.has(entity.raceId)) {
                return entity.name;
            }

            const raceName = raceNameMap.value[entity.raceId] || `unknownRace:${entity.raceId}`;
            if (entity instanceof GroupActor) {
                return raceName;
            }

            // for monster
            if (entity.name[0] >= '0' && entity.name[0] <= '9') {
                return `${raceName} (${entity.name.slice(-4)})`;
            }

            // for pet
            return entity.name;
        }

        const getChartSeriesData = (entity: EntityExtended[]): SeriesAreaOptions[] => {
            const series = entity.map((v): SeriesAreaOptions & { data: [number, number][] } => ({
                type: 'area',
                name: prettyEntityName(v.actor)!,
                data: v.damages.reduce((acc, v) => {
                    const normalizedTime = (v.At - (v.At % 60)) * 1000;
                    const last = acc[acc.length - 1];
                    if (last && last[0] == normalizedTime) {
                        last[1] += v.Damage;
                        return acc;
                    }

                    return acc.concat([[normalizedTime, v.Damage + (last?.[1] || 0)]]);
                }, [] as [number, number][]),
                stacking: 'normal',
                dataGrouping: {
                    enabled: true,
                    units: [['minute', [1]]],
                },
            }));

            for (let i = 0; i < (series[0]?.data.length || 0); i++) {
                const tickAt = Math.min(...series.map(v => v.data[i]?.[0] || Infinity));

                for (const s of series) {
                    if (s.data[i]?.[0] != tickAt) {
                        s.data.splice(i, 0, [tickAt, s.data[i - 1]?.[1] || 0]);
                    }
                }
            }

            return series;
        };

        return {
            isLoading,
            region,

            chartDom,
            pcEntities,
            allApplyDamage,
            targetId,
            targetIdList,
            detailDialog,
            detailDialogData,

            skillNameMap,
            condNameMap,
            entityMap: entityMapWithTargetData,

            condTooltip,
            condTooltipParent,
            condTooltipValue,
            setCondTooltip,

            showEntityDetailDamageList,
            getMabiNameColor,
            prettyEntityName,
        }
    }
});

type EntityExtended = {
    actor: EntityActor,
    dc: DualGroupedDamageCollector,
    totalDamage: number,
    damages: EntityDamage[],
    groupedTotalDamages: Record<string, number>,
    groupedDamages: Record<string, EntityDamage[]>,
    groupedMinDamages: Record<string, number>,
    groupedMaxDamages: Record<string, number>,
};

const chartOpt: Options = {
    lang: {
        locale: 'en',
    },
    title: { text: '' },
    chart: {
        zooming: { type: 'x' },
    },
    xAxis: { type: 'datetime' },
    yAxis: { title: { text: '' } },
    tooltip: {
        valueDecimals: 0,
    },
};

</script>