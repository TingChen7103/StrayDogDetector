<template>
    <v-sheet>
        <!-- <div ref="chartDom" style="width: 100svw; height: 300px;"></div> -->
    </v-sheet>
    <div class="d-flex ma-2 ga-2 align-center">
        <v-select v-model="targetId" :items="targetIdList"
            :item-title="vv => `${vv[0] ? prettyEntityName(entityMap[vv[0]]?.actor) : 'all'} ${vv[1]?.toFixed(0)}`"
            :item-value="vv => vv[0]" variant="outlined" density="compact" hide-details style="max-width: 65%;">
        </v-select>
        <v-text-field v-model.number="activeDpsBuffer" type="number" min="0" label="活躍DPS緩衝 (秒)"
            variant="outlined" density="compact" hide-details style="max-width: 35%;">
            <template v-slot:append-inner>
                <v-tooltip location="bottom" max-width="300">
                    <template v-slot:activator="{ props }">
                        <v-icon v-bind="props" size="small" color="grey">mdi-help-circle-outline</v-icon>
                    </template>
                    <span>活躍DPS僅計算傷害大於0的時段。若相鄰兩次攻擊間隔小於或等於緩衝時間（B 秒），會合併計算為同一次活躍攻擊區間。活躍區間僅向後延伸緩衝時間，且不超過對該敵人的最後一次攻擊時間。單次獨立攻擊的最少活躍時間為 1 秒。</span>
                </v-tooltip>
            </template>
        </v-text-field>
    </div>

    <v-expansion-panels multiple v-for="v in pcEntities" v-bind:key="v.actor.id">
        <template v-if="v.totalDamage > 0">
            <v-expansion-panel>
                <v-expansion-panel-title>
                    <v-sheet class="d-flex flex-wrap align-center bg-transparent" style="gap: 12px;">
                        <span class="font-weight-bold">{{ prettyEntityName(v.actor) }}</span>
                        <span>傷害: {{ v.totalDamage.toFixed(0) }}</span>
                        <span class="text-grey">({{ (100 * v.totalDamage / allApplyDamage).toFixed(1) }}%)</span>
                        <v-divider vertical class="mx-1"></v-divider>
                        <span>整體DPS: {{ v.damages.length < 2 ? 0 : Math.round(v.totalDamage / (v.damages[v.damages.length - 1].At - v.damages[0].At)) }}</span>
                        <v-divider vertical class="mx-1"></v-divider>
                        <span class="text-primary font-weight-bold">活躍DPS: {{ calculateActiveDps(v.damages, activeDpsBuffer) }}</span>
                    </v-sheet>
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
                                count: {{ v.groupedCount[+skillId] }}
                                avgDamage: {{ (v.groupedCount[+skillId] ? damageBySkill / v.groupedCount[+skillId] : 0).toFixed(0) }}
                                minDamage: {{ v.groupedMinDamages[+skillId]?.toFixed(0) || '0' }}
                                maxDamage: {{ v.groupedMaxDamages[+skillId]?.toFixed(0) || '0' }}
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
                <damage-list :attacker-name="detailDialogData?.attackerName || ''"
                :target-name="detailDialogData?.skillName || ''" :damages="detailDialogData?.damages || []" />
            </v-card-text>
            <v-card-actions>
                <v-btn color="primary" block @click="detailDialog = false">Close</v-btn>
            </v-card-actions>
        </v-card>
    </v-dialog>
</template>

<script lang="ts">
import { defineComponent, inject, ref, computed, onUnmounted, watch, onMounted } from "vue";
import highcharts, { Options, SeriesAreaOptions } from 'highcharts';

import { getMabiNameColor } from '@/util';
import { EntityDamage, ActorManager, BaseActor, GroupActor, EntityActor } from '@/eventActor';
import { DamageCollectorBase, DualGroupedDamageCollector, GroupedDamageCollector } from '@/actionCollector';

import DamageList from '@/components/subComponents/damageList.vue';

export default defineComponent({
    components: {
        DamageList,
    },
    setup() {
        const isLoading = inject('isLoading');
        const region = inject('region');
        const raceNameMap = inject('raceNameMap');
        const skillNameMap = inject('skillNameMap');
        const appEvent = inject('appEvent');
        const actorManager = inject('actorManager');
        const dcManager = inject('dcManager');

        const damageCollectorMap: Record<string, DamageCollectorBase> = {};
        const chartDom = ref<HTMLElement>(undefined!);
        let chart: Highcharts.Chart = undefined!;

        // const chartData = computed(() => getChartSeriesData(pcEntities.value.filter(v => v.totalDamage > 10000)));

        onUnmounted(() => {
            appEvent.value.removeEventListener('clear', clearTarget);

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
                const groupedCount = targetId.value ? v.dc.dualGroupedCount[targetId.value] : v.dc.grouped2Count;

                m[k] = {
                    ...v,
                    totalDamage,
                    damages,
                    groupedTotalDamages,
                    groupedDamages,
                    groupedMinDamages,
                    groupedMaxDamages,
                    groupedCount,
                }
            }

            return m;
        })

        const pcEntities = computed(() =>
            Object.values(entityMapWithTargetData.value).filter(v => v.actor.isPC).sort((a, b) => b.totalDamage - a.totalDamage));

        const targetId = ref('');
        const activeDpsBuffer = ref(10);

        const calculateActiveDps = (damages: EntityDamage[], buffer: number): number => {
            const validDamages = damages.filter(d => d.Damage > 0);
            if (validDamages.length === 0) {
                return 0;
            }

            // Sort by At ascending
            const sorted = [...validDamages].sort((a, b) => a.At - b.At);
            const b = Math.max(0, buffer || 0);
            const mergeThreshold = b;
            const tLast = sorted[sorted.length - 1].At;

            let activeTime = 0;
            let segmentStart = sorted[0].At;
            let segmentEnd = sorted[0].At;

            for (let i = 1; i < sorted.length; i++) {
                const currentAt = sorted[i].At;
                if (currentAt - segmentEnd <= mergeThreshold) {
                    segmentEnd = currentAt;
                } else {
                    // Finish current segment
                    const bufferedEnd = Math.min(segmentEnd + b, tLast);
                    const duration = Math.max(1, Math.ceil(bufferedEnd - segmentStart));
                    activeTime += duration;
                    // Start new segment
                    segmentStart = currentAt;
                    segmentEnd = currentAt;
                }
            }
            // Finish the last segment
            const bufferedEnd = Math.min(segmentEnd + b, tLast);
            const finalDuration = Math.max(1, Math.ceil(bufferedEnd - segmentStart));
            activeTime += finalDuration;

            const sumDamage = validDamages.reduce((sum, d) => sum + d.Damage, 0);
            return activeTime > 0 ? Math.round(sumDamage / activeTime) : 0;
        };
        const targetIdList = computed(() => {
            const list = Object.entries(targetDC.value.groupedTotalDamages)
                .sort(([, av], [, bv]) => bv - av);

            list.unshift(['', targetDC.value.totalDamage]);

            return list;
        });

        const clearTarget = () => {
            targetId.value = '';
        }

        onMounted(() => {
            appEvent.value.addEventListener('clear', clearTarget);

            // 20251228 Hans: 暫時不顯示圖表
            // let debounced1 = 0;
            // watch(entityMap, () => {
            //     if (debounced1) {
            //         return;
            //     }

            //     setTimeout(() => {
            //         debounced1 = 0;
            //         if (chart) {
            //             chart.destroy();
            //         }
            //         chart = highcharts.chart(chartDom.value, { ...chartOpt, series: chartData.value });
            //     }, 100);
            // });

            // let debounced2: number = 0;
            // watch(chartData, () => {
            //     if (debounced2) {
            //         return;
            //     }

            //     debounced2 = setTimeout(() => {
            //         debounced2 = 0;
            //         chart?.update({ series: chartData.value });
            //     }, 100);
            // });
        })

        const allApplyDamage = computed(() =>
            pcEntities.value.reduce((acc, v) => acc + v.totalDamage, 0));

        const detailDialog = ref(false);
        const detailDialogData = ref<{ attackerName: string; skillName: string; damages: EntityDamage[] }>();

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
            activeDpsBuffer,
            calculateActiveDps,
            detailDialog,
            detailDialogData,

            skillNameMap,
            entityMap: entityMapWithTargetData,

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
    groupedCount: Record<string, number>,
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