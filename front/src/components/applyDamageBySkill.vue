<template>
    <v-sheet v-show="targetId">
        <div ref="chartDom" style="width: 100svw; height: 300px;"></div>
    </v-sheet>

    <div class="d-flex ma-2 ga-2 align-center flex-wrap">
        <v-select v-model="targetId" :items="targetIdList"
            :item-title="vv => `${vv[0] ? prettyEntityName(entityMap[vv[0]]?.actor) : 'all'} ${vv[1]?.toFixed(0)}`"
            :item-value="vv => vv[0]" variant="outlined" density="compact" hide-details style="max-width: 400px; min-width: 200px;">
        </v-select>
        <v-text-field v-model.number="activeDpsBuffer" type="number" min="0" label="活躍DPS緩衝 (秒)"
            variant="outlined" density="compact" hide-details style="max-width: 150px; min-width: 100px;">
            <template v-slot:append-inner>
                <v-tooltip location="bottom" max-width="300">
                    <template v-slot:activator="{ props }">
                        <v-icon v-bind="props" size="small" color="grey">mdi-help-circle-outline</v-icon>
                    </template>
                    <span>活躍DPS僅計算傷害大於0的時段。若相鄰兩次攻擊間隔小於或等於緩衝時間（B 秒），會合併計算為同一次活躍攻擊區間。活躍區間僅向後延伸緩衝時間，且不超過對該敵人的最後一次攻擊時間。單次獨立攻擊的最少活躍時間為 1 秒。</span>
                </v-tooltip>
            </template>
        </v-text-field>
        <v-switch v-model="showBuffCoverage" label="顯示 Buff 覆蓋率" color="primary" density="compact" hide-details class="ml-2"></v-switch>
        <v-switch v-model="showDebuffCoverage" label="顯示 Debuff 覆蓋率" color="error" density="compact" hide-details class="ml-2"></v-switch>
        <v-btn v-if="targetId" color="primary" variant="outlined" density="compact" class="ml-2" @click="syncChart">
            圖表同步
        </v-btn>
        <span v-if="targetId" class="ml-2 font-weight-bold text-subtitle-2 text-grey-darken-3">
            攻略總時間: <span class="text-primary">{{ currentTargetDuration }}</span>
        </span>
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
                        <v-divider vertical class="mx-1"></v-divider>
                        <span class="text-deep-orange font-weight-bold">暴擊率: {{ calculateCriticalRate(v.damages) }}</span>
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
                                <div>
                                    {{ skillNameMap[+skillId] || `unknownSkill:${skillId}` }}
                                    damage: {{ damageBySkill.toFixed(0) }}
                                    count: {{ v.groupedCount[+skillId] }}
                                    avgDamage: {{ (v.groupedCount[+skillId] ? damageBySkill / v.groupedCount[+skillId] : 0).toFixed(0) }}
                                    minDamage: {{ v.groupedMinDamages[+skillId]?.toFixed(0) || '0' }}
                                    maxDamage: {{ v.groupedMaxDamages[+skillId]?.toFixed(0) || '0' }}
                                    {{ (100 * damageBySkill / v.totalDamage).toFixed(1) }}%
                                </div>
                                <div v-if="(showBuffCoverage || showDebuffCoverage) && getSkillConditionCoverage(v.groupedDamages[skillId], showBuffCoverage, showDebuffCoverage).length > 0" 
                                    class="d-flex flex-wrap align-center mt-1" style="gap: 8px;">
                                    <div v-for="c in getSkillConditionCoverage(v.groupedDamages[skillId], showBuffCoverage, showDebuffCoverage)" 
                                        v-bind:key="c.ccId" class="d-flex align-center bg-grey-darken-4 px-1 rounded text-caption" style="gap: 4px; border: 1px solid #444;">
                                        <img width="16" height="16" :src="`/res/characterconditionimage/${region}/${c.ccId}/${c.ccId}.png`" />
                                        <span>{{ condNameMap[c.ccId] || `CC:${c.ccId}` }}: <b>{{ c.percentage.toFixed(1) }}%</b></span>
                                    </div>
                                </div>
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
        const condNameMap = inject('condNameMap');

        const damageCollectorMap: Record<string, DamageCollectorBase> = {};
        const chartDom = ref<HTMLElement>(undefined!);
        let chart: Highcharts.Chart = undefined!;

        // chartData computed ref removed as we do on-demand chart sync

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

        const combatTimeRange = computed(() => {
            if (!targetId.value) {
                return { start: 0, end: 0, duration: 0 };
            }
            const dmgs = targetDC.value.groupedDamages[targetId.value] || [];
            if (dmgs.length === 0) {
                return { start: 0, end: 0, duration: 0 };
            }
            const validDmgs = dmgs.filter(d => d.Damage > 0);
            if (validDmgs.length === 0) {
                return { start: 0, end: 0, duration: 0 };
            }
            const times = validDmgs.map(d => d.At);
            const start = Math.min(...times);
            const end = Math.max(...times);
            return { start, end, duration: end - start };
        });

        const formatDuration = (seconds: number): string => {
            const s = Math.ceil(seconds);
            const m = Math.floor(s / 60);
            const remainingS = s % 60;
            return `${m}:${remainingS < 10 ? '0' : ''}${remainingS}`;
        };

        const currentTargetDuration = computed(() => {
            if (!targetId.value) {
                return '0:00';
            }
            const start = combatTimeRange.value.start;
            const end = combatTimeRange.value.end;
            if (start === 0 || end === 0) {
                return '0:00';
            }
            return formatDuration(end - start);
        });

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

        const calculateCriticalRate = (damages: EntityDamage[]): string => {
            if (!damages || damages.length === 0) {
                return '0.00%';
            }
            // 排除無暴擊設定的技能: 爆破(58100), 閃焰(58101), 疾風(58102, 50434), 追逐者(58103), 籠罩(58104), 連續攻擊(58009)
            const excludedSkills = [58100, 58101, 58102, 50434, 58103, 58104, 58009];
            const filteredDamages = damages.filter(d => !excludedSkills.includes(d.SkillId));
            if (filteredDamages.length === 0) {
                return '0.00%';
            }
            const criticalHits = filteredDamages.filter(d => d.IsCritical).length;
            const totalHits = filteredDamages.length;
            const rate = (criticalHits / totalHits) * 100;
            return `${rate.toFixed(2)}%`;
        };

        const showBuffCoverage = ref(false);
        const showDebuffCoverage = ref(false);

        const getSkillConditionCoverage = (damages: EntityDamage[] | undefined, showBuff: boolean, showDebuff: boolean) => {
            if (!damages || damages.length === 0 || (!showBuff && !showDebuff)) {
                return [];
            }

            const ccCountMap: Record<number, number> = {};
            const total = damages.length;

            for (const d of damages) {
                const checkedCCs = new Set<number>();
                if (showBuff && d.Conditions) {
                    for (const c of d.Conditions) {
                        checkedCCs.add(c.CCId);
                    }
                }
                if (showDebuff && d.TargetConditions) {
                    for (const c of d.TargetConditions) {
                        checkedCCs.add(c.CCId);
                    }
                }
                for (const ccId of checkedCCs) {
                    ccCountMap[ccId] = (ccCountMap[ccId] || 0) + 1;
                }
            }

            return Object.entries(ccCountMap)
                .map(([ccIdStr, count]) => {
                    const ccId = +ccIdStr;
                    return {
                        ccId,
                        percentage: (count / total) * 100,
                    };
                })
                .sort((a, b) => b.percentage - a.percentage);
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

        const syncChart = () => {
            if (!targetId.value || !chartDom.value) {
                return;
            }
            const latestSeries = getChartSeriesData(pcEntities.value.filter(v => v.totalDamage > 10000));
            if (chart) {
                chart.destroy();
                chart = undefined!;
            }
            chart = highcharts.chart(chartDom.value, { ...chartOpt, series: latestSeries });
        };

        onMounted(() => {
            appEvent.value.addEventListener('clear', clearTarget);

            watch(targetId, (newVal) => {
                if (!newVal) {
                    if (chart) {
                        chart.destroy();
                        chart = undefined!;
                    }
                } else {
                    setTimeout(() => {
                        syncChart();
                    }, 100);
                }
            });
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
            const start = combatTimeRange.value.start;
            const end = combatTimeRange.value.end;
            const duration = Math.ceil(end - start);

            // Build 1-second bins of damage for each player
            const playerBins = entity.map(v => {
                const dmgMap = new Map<number, number>();
                for (const d of v.damages) {
                    const sec = Math.floor(d.At - start);
                    if (sec >= 0) {
                        dmgMap.set(sec, (dmgMap.get(sec) || 0) + d.Damage);
                    }
                }
                return {
                    name: prettyEntityName(v.actor)!,
                    dmgMap
                };
            });

            // Calculate 5-second moving average for each second from 0 to duration
            const series = playerBins.map((p): SeriesAreaOptions & { data: [number, number][] } => {
                const data: [number, number][] = [[0, 0]]; // Start with [0, 0] at relative time 0

                for (let t = 1; t <= duration; t++) {
                    // Sum damages in window [t-4, t]
                    let windowSum = 0;
                    for (let w = Math.max(0, t - 4); w <= t; w++) {
                        windowSum += p.dmgMap.get(w) || 0;
                    }
                    const movingAverage = Math.round(windowSum / 5);
                    data.push([t * 1000, movingAverage]);
                }

                return {
                    type: 'area',
                    name: p.name,
                    data,
                    stacking: 'normal',
                };
            });

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
            calculateCriticalRate,
            showBuffCoverage,
            showDebuffCoverage,
            getSkillConditionCoverage,
            condNameMap,
            detailDialog,
            detailDialogData,

            skillNameMap,
            entityMap: entityMapWithTargetData,

            showEntityDetailDamageList,
            getMabiNameColor,
            prettyEntityName,
            currentTargetDuration,
            syncChart,
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
    xAxis: { 
        type: 'datetime',
        labels: {
            formatter: function (this: any) {
                const sec = Math.floor(this.value / 1000);
                const m = Math.floor(sec / 60);
                const s = sec % 60;
                return `${m}:${s < 10 ? '0' : ''}${s}`;
            }
        }
    },
    yAxis: { title: { text: '5秒移動平均傷害' } },
    tooltip: {
        valueDecimals: 0,
        shared: true,
        formatter: function (this: any) {
            const sec = Math.floor(this.x / 1000);
            const m = Math.floor(sec / 60);
            const s = sec % 60;
            const timeStr = `${m}:${s < 10 ? '0' : ''}${s}`;
            let tooltipText = `<b>相對時間: ${timeStr} (5秒移動平均)</b><br/>`;
            this.points.forEach((point: any) => {
                tooltipText += `<span style="color:${point.color}">\u25CF</span> ${point.series.name}: <b>${point.y.toLocaleString()}</b><br/>`;
            });
            return tooltipText;
        }
    },
};

</script>