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
                        v-for="[skillId, skillStats] in Object.entries(v.totalApplyDamageBySkill).sort((a, b) => b[1].damage - a[1].damage)"
                        v-bind:key="skillId" class="d-flex">
                        <v-sheet width="32" class="mr-2">
                            <img width="32" height="32" :src='`/res/skillimage/${region}/${skillId}/${skillId}.png`' />
                        </v-sheet>
                        <v-sheet width="100%" class="mb-4">
                            <!-- 스킬 이미지 -->
                            <!-- 이름 -->
                            <v-sheet width="100%" @click.stop="showEntityDetailDamageList(v.id, +skillId)">
                                {{ skillNameMap[+skillId] || `unknownSkill:${skillId}` }}
                                damage: {{ skillStats.damage.toFixed(0) }}
                                count: {{ skillStats.count }}
                                avgDamage: {{ (skillStats.damage / skillStats.count).toFixed(0) }}
                                {{ (100 * skillStats.damage / v.totalApplyDamage).toFixed(1) }}%
                            </v-sheet>

                            <!-- 막대 -->
                            <v-sheet width="100%" height="16">
                                <v-sheet @click.stop="showEntityDetailDamageList(v.id, +skillId)"
                                    :color="getMabiNameColor(skillNameMap[+skillId] || `unknownSkill:${skillId}`)"
                                    height="100%"
                                    :width="`${Math.round(100 * skillStats.damage / v.totalApplyDamage).toFixed(0)}%`"
                                    class="rounded-xl">
                                </v-sheet>
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
                                        {{ prettyEntityName(entityMap[item.TargetId]) }}
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

        const showEntityDetailDamageList = (attackerId: string, skillId: number) => {
            const entity = entityMap[attackerId];
            if (!entity) {
                return;
            }

            detailDialog.value = true;
            detailDialogData.value = {
                attackerName: prettyEntityName(entity)!,
                skillName: skillNameMap.value[skillId] || `unknownSkill:${skillId}`,
                damages: entity.applyDamages.filter(v => v.SkillId == skillId),
            };
        }

        const pcEntities = computed(() =>
            Object.values(entityMap).filter(v => v.isPC).sort((a, b) => b.totalApplyDamage - a.totalApplyDamage));

        const allApplyDamage = computed(() =>
            pcEntities.value.reduce((acc, v) => acc + v.totalApplyDamage, 0));

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