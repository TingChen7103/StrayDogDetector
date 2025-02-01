<template>
    <v-expansion-panels multiple
        v-for="[k, v] in Object.entries(groupMap).sort((a, b) => b[1].totalTakeDamage - a[1].totalTakeDamage)"
        v-bind:key="k">
        <template v-if="v.totalTakeDamage > 0">
            <v-expansion-panel>
                <v-expansion-panel-title>
                    <v-sheet>
                        {{ prettyEntityName(v) }} {{ v.totalTakeDamage.toFixed(0) }}
                    </v-sheet>

                </v-expansion-panel-title>
                <v-expansion-panel-text class="pa-3">
                    <template v-for="entity, entityk in v.entityMap" v-bind:key="entityk">
                        <template v-if="entity.totalTakeDamage > 0">
                            <v-sheet>
                                * {{ prettyEntityName(entity) }} {{ entity.totalTakeDamage.toFixed(0) }}
                                {{ entity.finisherId ? `Killed by
                                ${prettyEntityName(entityMap[entity.finisherId]) || entity.finisherId}` : '' }}
                                <template v-for="cond in entity.conditionMap" v-bind:key="cond.CCId">
                                    <img width="16" height="16"
                                        @mouseover="e => setCondTooltip(e.target! as HTMLElement, cond)"
                                        @mouseleave="e => setCondTooltip(e.target! as HTMLElement, undefined)"
                                        @click="e => setCondTooltip(e.target! as HTMLElement, cond)"
                                        :src='`http://localhost:${__api_port}/res/characterconditionimage/${region}/${cond.CCId}/${cond.CCId}.png`' />
                                </template>
                            </v-sheet>

                            <v-sheet
                                v-for="[attackerId, damageByAttacker] in Object.entries(entity.totalTakeDamageByAttacker).sort((a, b) => b[1] - a[1])"
                                v-bind:key="attackerId" width="100%" class="mb-4">
                                <!-- 이름 -->
                                <v-sheet width="100%" @click.stop="showEntityDetailDamageList(entity.id, attackerId)">
                                    {{ prettyEntityName(entityMap[attackerId]) || attackerId }} {{
                                        damageByAttacker.toFixed(0) }} {{
                                        (100 * damageByAttacker / entity.totalTakeDamage).toFixed(1) }}%
                                </v-sheet>

                                <!-- 막대 -->
                                <v-sheet width="100%" height="16">
                                    <v-sheet @click.stop="showEntityDetailDamageList(entity.id, attackerId)"
                                        :color="getMabiNameColor(prettyEntityName(entityMap[attackerId]) || attackerId)"
                                        height="100%"
                                        :width="`${Math.round(100 * damageByAttacker / entity.totalTakeDamage).toFixed(0)}%`"
                                        class="rounded-xl">
                                    </v-sheet>
                                </v-sheet>
                            </v-sheet>
                        </template>
                    </template>
                </v-expansion-panel-text>
            </v-expansion-panel>
            <v-sheet
                v-for="[attackerId, damageByAttacker] in Object.entries(v.totalTakeDamageByAttacker).sort((a, b) => b[1] - a[1])"
                v-bind:key="attackerId" width="100%" class="mb-4 pa-1">
                <!-- 이름 -->
                <v-sheet width="100%" @click.stop="showEntityGroupDetailDamageList(v.id, attackerId)">
                    {{ prettyEntityName(entityMap[attackerId]) || attackerId }} {{ damageByAttacker.toFixed(0) }} {{
                        (100 *
                            damageByAttacker /
                            v.totalTakeDamage).toFixed(1) }}%
                </v-sheet>

                <!-- 막대 -->
                <v-sheet width="100%" height="16">
                    <v-sheet @click.stop="showEntityGroupDetailDamageList(v.id, attackerId)"
                        :color="getMabiNameColor(prettyEntityName(entityMap[attackerId]) || attackerId)" height="100%"
                        :width="`${Math.round(100 * damageByAttacker / v.totalTakeDamage).toFixed(0)}%`"
                        class="rounded-xl">
                    </v-sheet>
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
                                        :src='`http://localhost:${__api_port}/res/characterconditionimage/${region}/${cond.CCId}/${cond.CCId}.png`' />
                                </template>
                                ->
                                <template v-for="cond in item.TargetConditions" v-bind:key="cond.CCId">
                                    <img width="16" height="16"
                                        @mouseover="e => setCondTooltip(e.target! as HTMLElement, cond)"
                                        @mouseleave="e => setCondTooltip(e.target! as HTMLElement, undefined)"
                                        @click="e => setCondTooltip(e.target! as HTMLElement, cond)"
                                        :src='`http://localhost:${__api_port}/res/characterconditionimage/${region}/${cond.CCId}/${cond.CCId}.png`' />
                                </template>
                            </p>
                            <p>
                                <img width="32" height="32"
                                    :src='`http://localhost:${__api_port}/res/skillimage/${region}/${item.SkillId}/${item.SkillId}.png`' />
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
import { defineComponent, inject, ref } from "vue";

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

        const showEntityDetailDamageList = (targetId: string, attackerId: string) => {
            const entity = entityMap[targetId];
            if (!entity) {
                return;
            }

            detailDialog.value = true;
            detailDialogData.value = {
                targetName: prettyEntityName(entity)!,
                attackerName: prettyEntityName(entityMap[attackerId]) || attackerId,
                Damages: entity.takeDamages.filter(v => v.Id == attackerId),
            };
        }

        const showEntityGroupDetailDamageList = (targetId: string, attackerId: string) => {
            const group = groupMap[targetId];
            if (!group) {
                return;
            }

            detailDialog.value = true;
            detailDialogData.value = {
                targetName: prettyEntityName(group)!,
                attackerName: prettyEntityName(entityMap[attackerId]) || attackerId,
                Damages: group.takeDamages.filter(v => v.Id == attackerId),
            };
        }

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
            __api_port,
            isLoading,
            region,

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
            showEntityGroupDetailDamageList,
            getMabiNameColor,
            prettyEntityName,
        }
    }
});

</script>