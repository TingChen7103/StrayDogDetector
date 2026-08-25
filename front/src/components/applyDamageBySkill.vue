<template>
    <v-sheet v-show="allApplyDamage > 0" class="position-relative">
        <v-overlay
            :model-value="isChartLoading"
            contained
            persistent
            class="align-center justify-center"
            scrim="#121212"
            opacity="0.8"
        >
            <div class="text-h4 font-weight-bold text-grey-lighten-1">Loading...</div>
        </v-overlay>
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
        <v-text-field v-model.number="chartWindowSize" type="number" min="1" label="圖表窗格 (秒)"
            variant="outlined" density="compact" hide-details style="max-width: 150px; min-width: 100px;">
        </v-text-field>
        <v-switch v-model="showBuffCoverage" label="顯示 Buff 覆蓋率" color="primary" density="compact" hide-details class="ml-2"></v-switch>
        <v-btn v-if="showBuffCoverage" :color="showRawBuffCoverage ? 'warning' : 'grey'"
            :variant="showRawBuffCoverage ? 'flat' : 'outlined'" density="compact" class="ml-2"
            @click="showRawBuffCoverage = !showRawBuffCoverage">
            {{ showRawBuffCoverage ? '原始狀態 (未過濾)' : '白名單過濾中' }}
            <v-tooltip activator="parent" location="bottom" max-width="360">
                <span>白名單只顯示常用的 Buff。切到「原始狀態」可看到全部未過濾的條件，用來檢查有沒有漏掉該加進白名單的項目。白名單設定在 front/src/conditionWhitelist.ts。</span>
            </v-tooltip>
        </v-btn>
        <v-switch v-model="showDebuffCoverage" label="顯示完整Debuff覆蓋率" color="error" density="compact" hide-details class="ml-2">
            <v-tooltip activator="parent" location="bottom" max-width="360">
                <span>預設只顯示白名單 Debuff 的圖示與覆蓋率。開啟後會顯示名稱與 CCId，並把白名單外的所有 Debuff 接在後面，用來檢查有沒有漏掉該加進白名單的項目。白名單設定在 front/src/conditionWhitelist.ts。</span>
            </v-tooltip>
        </v-switch>
        <v-btn v-if="allApplyDamage > 0" color="primary" variant="outlined" density="compact" class="ml-2" @click="syncChart">
            圖表同步
        </v-btn>
        <span v-if="allApplyDamage > 0" class="ml-2 font-weight-bold text-subtitle-2 text-grey-darken-3">
            攻略總時間: <span class="text-primary">{{ currentTargetDuration }}</span>
        </span>
    </div>

    <!-- Debuff 覆蓋率:選定敵人後自動顯示,不跟著技能列 -->
    <v-sheet v-if="targetId" class="mx-2 mb-2 px-3 py-2 rounded" color="grey-darken-4" border>
        <div class="d-flex align-center flex-wrap" style="gap: 10px;">
            <span class="text-caption font-weight-bold text-error" style="white-space: nowrap;">
                Debuff 覆蓋率
            </span>
            <span class="text-caption text-grey" style="white-space: nowrap;">
                {{ prettyEntityName(entityMap[targetId]?.actor) }}
            </span>

            <template v-if="debuffCoverage.length > 0">
                <div v-for="c in debuffCoverage" v-bind:key="c.ccId"
                    class="d-flex align-center bg-grey-darken-3 px-1 rounded text-caption"
                    :style="`gap: 4px; border: 1px solid ${isWhitelistedDebuff(c.ccId) ? '#a33' : '#444'};`
                        + (c.percentage === 0 ? ' opacity: 0.4;' : '')">
                    <img width="16" height="16" :src="`/res/characterconditionimage/${region}/${c.ccId}/${c.ccId}.png`"
                        :style="c.percentage === 0 ? 'filter: grayscale(1);' : ''" />
                    <span v-if="showDebuffCoverage" class="text-grey-lighten-1">
                        {{ condNameMap[c.ccId] || `CC:${c.ccId}` }}:
                    </span>
                    <b :class="c.percentage === 0 ? 'text-grey' : ''">{{ c.percentage.toFixed(1) }}%</b>
                    <v-tooltip v-if="!showDebuffCoverage" activator="parent" location="bottom">
                        <span>{{ condNameMap[c.ccId] || `CC:${c.ccId}` }}{{ c.percentage === 0 ? '（此敵人未使用）' : '' }}</span>
                    </v-tooltip>
                </div>
            </template>
            <span v-else class="text-caption text-grey-darken-1">（此敵人無符合的 Debuff 紀錄）</span>
        </div>
    </v-sheet>

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
                                    <span class="font-weight-bold">{{ formatSkillName(+skillId) }}</span>
                                    <span class="ml-2 text-grey-darken-1">damage:</span> <span class="font-weight-bold">{{ damageBySkill.toFixed(0) }}</span>
                                    <span class="ml-2 text-grey-darken-1">count:</span> <span class="font-weight-bold">{{ v.groupedCount[+skillId] }}</span>
                                    <span class="ml-2 text-orange-darken-2">avgDamage: {{ (v.groupedCount[+skillId] ? damageBySkill / v.groupedCount[+skillId] : 0).toFixed(0) }}</span>
                                    <span class="ml-2 text-grey-darken-1">minDamage: {{ v.groupedMinDamages[+skillId]?.toFixed(0) || '0' }}</span>
                                    <span class="ml-2 text-blue-darken-1">maxDamage: {{ v.groupedMaxDamages[+skillId]?.toFixed(0) || '0' }}</span>
                                    <span class="ml-2 text-grey-darken-1">傷害佔比:</span> <span class="font-weight-bold">{{ (100 * damageBySkill / v.totalDamage).toFixed(1) }}%</span>
                                    <span class="ml-2 text-error font-weight-bold" v-if="!isExcludedSkill(+skillId)">暴擊率: {{ calculateSkillCriticalRate(v.groupedDamages[+skillId]) }}</span>
                                </div>
                                <div v-if="showBuffCoverage && getSkillConditionCoverage(v.groupedDamages[skillId], showBuffCoverage, showRawBuffCoverage).length > 0"
                                    class="d-flex flex-wrap align-center mt-1" style="gap: 8px;">
                                    <div v-for="c in getSkillConditionCoverage(v.groupedDamages[skillId], showBuffCoverage, showRawBuffCoverage)"
                                        v-bind:key="c.label || c.ccId" class="d-flex align-center bg-grey-darken-4 px-1 rounded text-caption" style="gap: 4px; border: 1px solid #444;">
                                        <img width="16" height="16" :src="`/res/characterconditionimage/${region}/${c.ccId}/${c.ccId}.png`" />
                                        <span>{{ c.label || condNameMap[c.ccId] || `CC:${c.ccId}` }}: <b>{{ c.percentage.toFixed(1) }}%</b></span>

                                        <!-- 音樂 buff: 顯示該次施放的實際加成數值與其佔生效期間的比例 -->
                                        <v-tooltip v-if="hasMusicBuffDisplay(c.ccId)" activator="parent" location="bottom" max-width="420">
                                            <div class="text-caption font-weight-bold mb-1">
                                                {{ condNameMap[c.ccId] || `CC:${c.ccId}` }} — 生效期間的加成數值
                                            </div>
                                            <div v-for="b in computeMusicBuffBreakdown(v.groupedDamages[skillId], c.ccId)"
                                                v-bind:key="b.text" class="d-flex justify-space-between" style="gap: 12px;">
                                                <span>{{ b.text }}</span>
                                                <b>{{ b.percentage.toFixed(1) }}%</b>
                                            </div>
                                            <div class="text-caption text-grey mt-1">
                                                此處百分比相加為 100%（分母為該 buff 生效的傷害次數）
                                            </div>
                                        </v-tooltip>
                                    </div>
                                </div>
                            </v-sheet>

                            <!-- 막대 -->
                            <v-sheet width="100%" height="16">
                                <v-sheet @click.stop="showEntityDetailDamageList(v.actor.id, '', +skillId)"
                                    :color="getMabiNameColor(formatSkillName(+skillId))"
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
import {
    isWhitelistedBuff,
    statusSupportIds,
    statusSupportMergeThreshold,
    statusSupportDisplayId,
    statusSupportDisplayName,
    isWhitelistedDebuff,
    computeDebuffCoverage,
    DebuffCoverageEntry,
    hasMusicBuffDisplay,
    computeMusicBuffBreakdown,
} from '@/conditionWhitelist';

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

        const targetId = ref('');
        const activeDpsBuffer = ref(10);
        const chartWindowSize = ref(5);

        // chartData computed ref removed as we do on-demand chart sync

        const isExcludedSkill = (skillId: number): boolean => {
            const excludedSkills = [58100, 58101, 58102, 50434, 58103, 58104, 58009];
            return excludedSkills.includes(skillId);
        };

        const calculateSkillCriticalRate = (damages: EntityDamage[] | undefined): string => {
            if (!damages || damages.length === 0) {
                return '0.00%';
            }
            const criticalHits = damages.filter(d => d.IsCritical).length;
            const totalHits = damages.length;
            const rate = (criticalHits / totalHits) * 100;
            return `${rate.toFixed(2)}%`;
        };

        let isSettingsLoaded = false;
        const loadSettings = async () => {
            try {
                const res = await fetch('/api/settings');
                if (res.ok) {
                    const data = await res.json();
                    if (data.ActiveDpsBuffer) {
                        activeDpsBuffer.value = parseInt(data.ActiveDpsBuffer, 10) || 10;
                    }
                    if (data.ChartWindowSize) {
                        chartWindowSize.value = parseInt(data.ChartWindowSize, 10) || 5;
                    }
                }
            } catch (e) {
                console.error("Failed to load settings:", e);
            } finally {
                isSettingsLoaded = true;
            }
        };

        watch([activeDpsBuffer, chartWindowSize], async ([newBuffer, newWindowSize]) => {
            if (!isSettingsLoaded) return;
            try {
                await fetch('/api/settings', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        ActiveDpsBuffer: String(newBuffer),
                        ChartWindowSize: String(newWindowSize)
                    })
                });
            } catch (e) {
                console.error("Failed to save settings:", e);
            }
        });

        const handleClear = () => {
            clearTarget();
            for (const k in damageCollectorMap) {
                if (k === 'target') continue;
                const v = damageCollectorMap[k];
                dcManager.value.removeDamageCollector(v);
                delete damageCollectorMap[k];
            }
        };

        onUnmounted(() => {
            appEvent.value.removeEventListener('clear', handleClear);

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
                skillName: formatSkillName(skillId),
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
            const dmgs = !targetId.value 
                ? (targetDC.value.damages || []) 
                : (targetDC.value.groupedDamages[targetId.value] || []);
            if (dmgs.length === 0) {
                return { start: 0, end: 0, duration: 0 };
            }
            let start = Infinity;
            let end = -Infinity;
            let hasValid = false;
            for (let i = 0; i < dmgs.length; i++) {
                const d = dmgs[i];
                if (d.Damage > 0) {
                    hasValid = true;
                    if (d.At < start) start = d.At;
                    if (d.At > end) end = d.At;
                }
            }
            if (!hasValid) {
                return { start: 0, end: 0, duration: 0 };
            }
            return { start, end, duration: end - start };
        });

        const formatDuration = (seconds: number): string => {
            const s = Math.ceil(seconds);
            const m = Math.floor(s / 60);
            const remainingS = s % 60;
            return `${m}:${remainingS < 10 ? '0' : ''}${remainingS}`;
        };

        const currentTargetDuration = computed(() => {
            const start = combatTimeRange.value.start;
            const end = combatTimeRange.value.end;
            if (start === 0 || end === 0) {
                return '0:00';
            }
            return formatDuration(end - start);
        });

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
        // 顯示未經白名單過濾的原始狀態,用來檢查有沒有漏掉的條件
        const showRawBuffCoverage = ref(false);

        type ConditionCoverage = {
            ccId: number;
            percentage: number;
            /** 合併項的顯示名稱;未設定時沿用 condNameMap */
            label?: string;
        };

        // 技能列只顯示 Buff 覆蓋率;Debuff 已改由下方獨立區域統一顯示
        const getSkillConditionCoverage = (
            damages: EntityDamage[] | undefined,
            showBuff: boolean,
            showRaw: boolean,
        ): ConditionCoverage[] => {
            if (!damages || damages.length === 0 || !showBuff) {
                return [];
            }

            const ccCountMap: Record<number, number> = {};
            const total = damages.length;
            // 狀態支援群組的聯集次數 (任一個在線就算),供合併列使用
            let statusSupportCount = 0;

            for (const d of damages) {
                const checkedCCs = new Set<number>();
                if (showBuff && d.Conditions) {
                    for (const c of d.Conditions) {
                        if (!showRaw && !isWhitelistedBuff(c.CCId, condNameMap.value[c.CCId])) {
                            continue;
                        }
                        checkedCCs.add(c.CCId);
                    }
                }
                for (const ccId of checkedCCs) {
                    ccCountMap[ccId] = (ccCountMap[ccId] || 0) + 1;
                }

                if (!showRaw && statusSupportIds.some(id => checkedCCs.has(id))) {
                    statusSupportCount++;
                }
            }

            let entries: { ccId: number; count: number; label?: string }[] =
                Object.entries(ccCountMap).map(([ccIdStr, count]) => ({ ccId: +ccIdStr, count }));

            if (!showRaw) {
                // 銳利/再生/迅速/逆轉 同時出現 3 個以上時合併成一列「狀態支援」
                const presentCount = statusSupportIds.filter(id => ccCountMap[id] !== undefined).length;
                if (presentCount >= statusSupportMergeThreshold) {
                    entries = entries.filter(e => !statusSupportIds.includes(e.ccId));
                    entries.push({
                        ccId: statusSupportDisplayId,
                        count: statusSupportCount,
                        label: statusSupportDisplayName,
                    });
                }
            }

            return entries
                .map(e => ({
                    ccId: e.ccId,
                    percentage: (e.count / total) * 100,
                    label: e.label,
                }))
                .sort((a, b) => b.percentage - a.percentage);
        };

        /**
         * 選定敵人的 Debuff 覆蓋率。
         *
         * 基準與 Buff 覆蓋率一致:對該敵人的所有傷害事件中,
         * 有幾成發生的當下該 debuff 掛在牠身上。
         * 白名單的合併組 (912/913、504/392) 以聯集計算 —— 任一個在身上就算。
         * 白名單項目依 conditionWhitelist 的順序排在前面;
         * 開啟「顯示完整Debuff覆蓋率」時,白名單外的項目依覆蓋率高低接在後面。
         */
        const debuffCoverage = computed((): DebuffCoverageEntry[] => {
            if (!targetId.value) {
                // 未指定特定敵人時不顯示 (多隻敵人混在一起數字沒有意義)
                return [];
            }

            return computeDebuffCoverage(
                targetDC.value.groupedDamages[targetId.value] || [],
                showDebuffCoverage.value,
            );
        });

        const targetIdList = computed(() => {
            const list = Object.entries(targetDC.value.groupedTotalDamages)
                .sort(([, av], [, bv]) => bv - av);

            list.unshift(['', targetDC.value.totalDamage]);

            return list;
        });

        const clearTarget = () => {
            targetId.value = '';
        }

        let currentSyncId = 0;
        const isChartLoading = ref(false);

        const syncChart = async () => {
            const mySyncId = ++currentSyncId;

            if (!chartDom.value || allApplyDamage.value === 0 || combatTimeRange.value.duration <= 0) {
                if (chart) {
                    chart.destroy();
                    chart = undefined!;
                }
                isChartLoading.value = false;
                return;
            }

            isChartLoading.value = true;
            // 延遲以允許 DOM 更新並顯示 Loading 遮罩
            await new Promise(resolve => setTimeout(resolve, 50));

            if (mySyncId !== currentSyncId) {
                return;
            }

            try {
                const latestSeries = getChartSeriesData(pcEntities.value.filter(v => v.totalDamage > 10000), chartWindowSize.value);
                if (mySyncId !== currentSyncId) {
                    return;
                }

                if (chart) {
                    chart.destroy();
                    chart = undefined!;
                }
                // 戰鬥起始的絕對時間 (unix 秒),用來把相對時間換算成使用者系統時間
                const combatStart = combatTimeRange.value.start;

                const dynamicChartOpt: Options = {
                    ...chartOpt,
                    xAxis: [
                        {
                            // 下方: 相對時間 (維持原本行為)
                            type: 'datetime',
                            labels: {
                                formatter: function (this: any) {
                                    return formatRelTime(this.value);
                                }
                            }
                        },
                        {
                            // 上方: 使用者系統時間,與下方軸共用刻度並排顯示
                            type: 'datetime',
                            linkedTo: 0,
                            opposite: true,
                            title: { text: '系統時間' },
                            labels: {
                                formatter: function (this: any) {
                                    return formatClockTime(this.value, combatStart);
                                }
                            }
                        },
                    ],
                    yAxis: {
                        title: { text: `${chartWindowSize.value}秒 DPS` }
                    },
                    tooltip: {
                        valueDecimals: 0,
                        shared: true,
                        formatter: function (this: any) {
                            const timeStr = formatRelTime(this.x);
                            const clockStr = formatClockTime(this.x, combatStart);
                            let tooltipText = `<b>相對時間: ${timeStr}`;
                            if (clockStr) {
                                tooltipText += `　系統時間: ${clockStr}`;
                            }
                            tooltipText += ` (${chartWindowSize.value}秒 DPS)</b><br/>`;
                            this.points.forEach((point: any) => {
                                tooltipText += `<span style="color:${point.color}">\u25CF</span> ${point.series.name}: <b>${point.y.toLocaleString()}</b><br/>`;
                            });
                            return tooltipText;
                        }
                    },
                    series: latestSeries
                };
                chart = highcharts.chart(chartDom.value, dynamicChartOpt);
                // 額外延遲，確保瀏覽器在關閉遮罩前已完成圖表繪製渲染
                await new Promise(resolve => setTimeout(resolve, 150));
            } finally {
                if (mySyncId === currentSyncId) {
                    isChartLoading.value = false;
                }
            }
        };

        onMounted(() => {
            appEvent.value.addEventListener('clear', handleClear);
            loadSettings();

            // 使用 flush: 'post' 確保在 DOM 更新完成後才觸發，避免 initial render 時 ref 尚未綁定
            watch([targetId, pcEntities, chartWindowSize], () => {
                syncChart();
            }, { immediate: true, flush: 'post' });
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

        const formatSkillName = (skillId: number) => {
            return skillNameMap.value[+skillId] || `unknownSkill:${skillId}`;
        };

        const getChartSeriesData = (entity: EntityExtended[], windowSize: number): any[] => {
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

            // Calculate moving average for each second from 0 to duration
            const series = playerBins.map((p): any => {
                const data: [number, number][] = [[0, 0]]; // Start with [0, 0] at relative time 0

                for (let t = 1; t <= duration; t++) {
                    // Sum damages in window [t - (windowSize - 1), t]
                    let windowSum = 0;
                    for (let w = Math.max(0, t - (windowSize - 1)); w <= t; w++) {
                        windowSum += p.dmgMap.get(w) || 0;
                    }
                    const movingAverage = Math.round(windowSum / windowSize);
                    data.push([t * 1000, movingAverage]);
                }

                return {
                    type: 'line',
                    name: p.name,
                    data,
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
            chartWindowSize,
            calculateActiveDps,
            calculateCriticalRate,
            showBuffCoverage,
            showDebuffCoverage,
            showRawBuffCoverage,
            getSkillConditionCoverage,
            debuffCoverage,
            isWhitelistedDebuff,
            hasMusicBuffDisplay,
            computeMusicBuffBreakdown,
            condNameMap,
            detailDialog,
            detailDialogData,

            skillNameMap,
            entityMap: entityMapWithTargetData,

            showEntityDetailDamageList,
            getMabiNameColor,
            prettyEntityName,
            currentTargetDuration,
            isChartLoading,
            syncChart,
            formatSkillName,
            isExcludedSkill,
            calculateSkillCriticalRate,
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

/** 相對時間 (圖表 X 軸的毫秒) -> M:SS */
const formatRelTime = (relMs: number): string => {
    const sec = Math.floor(relMs / 1000);
    const m = Math.floor(sec / 60);
    const s = sec % 60;
    return `${m}:${s < 10 ? '0' : ''}${s}`;
};

/**
 * 相對時間 -> 使用者系統時間 HH:MM:SS
 * startSec 為戰鬥起始的 unix 秒;Date 以瀏覽器本機時區呈現,即使用者的系統時間。
 */
const formatClockTime = (relMs: number, startSec: number): string => {
    if (!startSec) {
        return '';
    }

    const d = new Date((startSec * 1000) + relMs);
    const pad = (n: number) => (n < 10 ? '0' : '') + n;

    return `${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
};

const chartOpt: Options = {
    lang: {
        locale: 'en',
    },
    title: { text: '' },
    chart: {
        zooming: { type: 'x' },
        animation: false, // 關閉圖表整體載入動畫以提升效能
    },
    plotOptions: {
        series: {
            animation: false, // 關閉數據線條/面積動畫，避免大數據時背景渲染卡頓
        },
    },
    // 註: syncChart 會以 dynamicChartOpt 覆寫 xAxis / tooltip (加上系統時間軸),
    //     這裡的設定僅作為基底預設值
    xAxis: {
        type: 'datetime',
        labels: {
            formatter: function (this: any) {
                return formatRelTime(this.value);
            }
        }
    },
    yAxis: { title: { text: '5秒移動平均傷害' } },
    tooltip: {
        valueDecimals: 0,
        shared: true,
        formatter: function (this: any) {
            const timeStr = formatRelTime(this.x);
            let tooltipText = `<b>相對時間: ${timeStr} (5秒移動平均)</b><br/>`;
            this.points.forEach((point: any) => {
                tooltipText += `<span style="color:${point.color}">\u25CF</span> ${point.series.name}: <b>${point.y.toLocaleString()}</b><br/>`;
            });
            return tooltipText;
        }
    },
};

</script>