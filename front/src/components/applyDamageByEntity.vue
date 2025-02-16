<template>
    <v-expansion-panels multiple v-for="v in pcEntities" v-bind:key="v.id">
        <template v-if="v.totalApplyDamage > 0">
            <v-expansion-panel>
                <v-expansion-panel-title>
                    <v-sheet>
                        {{ prettyEntityName(v) }} {{ v.totalApplyDamage.toFixed(0) }}
                    </v-sheet>

                </v-expansion-panel-title>
                <v-expansion-panel-text class="pa-3">
                    <v-sheet
                        v-for="[targetId, damageToTarget] in Object.entries(v.totalApplyDamageByTarget).sort((a, b) => b[1] - a[1])"
                        v-bind:key="targetId" width="100%" class="mb-4">
                        <!-- 이름 -->
                        <v-sheet width="100%" @click.stop="showEntityDetailDamageList(v.id, targetId)">
                            {{ prettyEntityName(entityMap[targetId]) || targetId }} {{
                                damageToTarget.toFixed(0) }} {{
                                (100 * damageToTarget / v.totalApplyDamage).toFixed(1) }}%
                        </v-sheet>

                        <!-- 막대 -->
                        <v-sheet width="100%" height="16">
                            <v-sheet @click.stop="showEntityDetailDamageList(v.id, targetId)"
                                :color="getMabiNameColor(prettyEntityName(entityMap[targetId]) || targetId)"
                                height="100%"
                                :width="`${Math.round(100 * damageToTarget / v.totalApplyDamage).toFixed(0)}%`"
                                class="rounded-xl">
                            </v-sheet>
                        </v-sheet>
                    </v-sheet>
                </v-expansion-panel-text>
            </v-expansion-panel>
            <!-- 막대 -->
            <v-sheet width="100%" height="16">
                <v-sheet :color="getMabiNameColor(prettyEntityName(entityMap[v.id]) || v.id)" height="100%"
                    :width="`${Math.round(100 * v.totalApplyDamage / allApplyDamage).toFixed(0)}%`" class="rounded-xl">
                </v-sheet>
            </v-sheet>
        </template>

    </v-expansion-panels>

    <v-dialog v-model="detailDialog" min-width="60vw" height="90svh">
        <v-card>
            <v-card-text class="pa-0">
                <v-sheet width="100%" class="d-flex pa-2 mb-2">
                    {{ detailDialogData?.attackerName }} -> {{ detailDialogData?.targetName }}
                </v-sheet>

                <v-virtual-scroll :items="detailDialogData?.Damages"
                    style="min-height: 300px; height: calc(90svh - 200px)" item-height="80">
                    <template v-slot:default="{ item }">
                        <v-card min-height="80">
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
                                <img width="32" height="32"
                                    :src='`/res/skillimage/${region}/${item.SkillId}/${item.SkillId}.png`' />
                                {{ skillNameMap[item.SkillId] }} {{ item.Damage.toFixed(0) }} {{ item.IsCritical ?
                                    '크리티컬' : '' }}
                            </p>
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
import { defineComponent, inject, ref, computed } from "vue";

import { getMabiNameColor } from '@/util';
import { EntityDamage, EntityCondition, ActorManager, BaseActor, GroupActor } from '@/eventActor';

export default defineComponent({
    setup() {
        const isLoading = inject('isLoading');
        const region = inject('region');
        const raceNameMap = inject('raceNameMap');
        const skillNameMap = inject('skillNameMap');
        const condNameMap = inject('condNameMap');
        const actorManager = inject('actorManager');

        const entityMap = actorManager.value.entityMap;
        const groupMap = actorManager.value.groupMap;

        const showEntityDetailDamageList = (attackerId: string, targetId: string) => {
            const entity = entityMap[attackerId];
            if (!entity) {
                return;
            }

            detailDialog.value = true;
            detailDialogData.value = {
                targetName: prettyEntityName(entityMap[targetId]) || targetId,
                attackerName: prettyEntityName(entity)!,
                Damages: entity.applyDamages.filter(v => v.TargetId == targetId),
            };
        }

        const pcEntities = computed(() =>
            Object.values(entityMap).filter(v => v.isPC).sort((a, b) => b.totalApplyDamage - a.totalApplyDamage));

        const allApplyDamage = computed(() =>
            pcEntities.value.reduce((acc, v) => acc + v.totalApplyDamage, 0));

        const detailDialog = ref(false);
        const detailDialogData = ref<{ targetName: string; attackerName: string; Damages: EntityDamage[] }>();

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

        return {
            isLoading,
            region,

            pcEntities,
            allApplyDamage,
            detailDialog,
            detailDialogData,

            skillNameMap,
            condNameMap,
            entityMap,
            groupMap,

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

</script>