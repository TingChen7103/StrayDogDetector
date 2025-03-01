import { shallowReactive } from 'vue';
import * as bounds from 'binary-search-bounds';

import * as protocols from '@/protocols';
import { DamageCollectorManager } from '@/actionCollector';

// TODO: take cc, apply cc 구분하기

export class ActorManager {
    constructor(private _damageCollector: DamageCollectorManager) {
    }

    public entityMap: Record<string, EntityActor> = shallowReactive({});
    public groupMap: Record<string, GroupActor> = shallowReactive({});
    public damages: protocols.eventDamage[] = [];

    public static pcRaceSet = new Set<number>([8001, 8002, 9001, 9002, 10001, 10002]);

    public onEvent(event: protocols.eventBase) {
        if (event.EventId === protocols.eventIdEntityAppear) {
            this.onEntityAppear(event as protocols.eventEntityAppear);
        }

        const entity = this.entityMap[event.Id];

        switch (event.EventId) {
            case protocols.eventIdEntityAppear:
                // TODO: entityAppear에서 master id, hp 가져오기, damage, cc 처리할 때 master id가 있으면 그쪽으로 보내야함
                entity.onEntityAppear(event as protocols.eventEntityAppear);
                break;

            case protocols.eventIdDamage:
                {
                    const event_ = event as protocols.eventDamage;
                    this.damages.push(event_);

                    if (entity) {
                        entity.onApplyDamage(event_);
                        entity.group.onApplyDamage(event_);
                    }

                    const targetEntity = this.entityMap[event_.TargetId];
                    if (targetEntity) {
                        targetEntity.onTakeDamage(event_);
                        targetEntity.group.onTakeDamage(event_);
                    }
                }
                break;

            case protocols.eventIdCharacterConditionEnable:
                if (!entity) {
                    return;
                }

                entity.onCharacterConditionEnable(event as protocols.eventCharacterConditionEnable);
                break;

            case protocols.eventIdCharacterConditionDisable:
                if (!entity) {
                    return;
                }

                entity.onCharacterConditionDisable(event as protocols.eventCharacterConditionDisable);
                break;

            case protocols.eventIdFinish:
                if (!entity) {
                    return;
                }

                entity.onFinish(event as protocols.eventFinish);
                break;

            case protocols.eventIdEntityEquipItem:
                if (!entity) {
                    return;
                }

                entity.onEquipItem(event as protocols.eventEntityEquipItem);
                break;

            case protocols.eventIdEntityUnequipItem:
                if (!entity) {
                    return;
                }

                entity.onUnequipItem(event as protocols.eventEntityUnequipItem);
                break;

            case protocols.eventIdEntityUpdateBody:
                if (!entity) {
                    return;
                }

                entity.onUpdateBody(event as protocols.eventEntityUpdateBody);
                break;
        }
    }

    public onEntityAppear(event: protocols.eventEntityAppear) {
        const { Id, RaceId, Name } = event;

        const groupKey = ActorManager.groupTargetKey(event);
        const group = this.groupMap[groupKey] ??= new GroupActor(this, groupKey, RaceId, Name);

        const isNewEntity = !this.entityMap[Id];

        if (isNewEntity) {
            const entity = new EntityActor(this, Id, RaceId, Name, group);
            this.entityMap[Id] = group.entityMap[Id] = entity;

            // entity appear를 받은 뒤에 api가 켜진 경우
            for (const v of this.damages) {
                if (v.Id == Id) {
                    entity.onApplyDamage(v);
                    entity.group.onApplyDamage(v);
                }
                else if (v.TargetId == Id) {
                    entity.onTakeDamage(v);
                    entity.group.onTakeDamage(v);
                }
            }
        }
    }

    public onEntityDamage(event: EntityDamage) {
        this._damageCollector.onDamage(event);
    }

    public clear() {
        // object instance를 새로 만들면 귀찮아짐
        this.damages.length = 0;

        for (const k in this.entityMap) {
            const v = this.entityMap[k];

            v.clear();
        }

        for (const k in this.groupMap) {
            const v = this.groupMap[k];

            v.clear();
        }
    }

    private static groupTargetKey(event: protocols.eventEntityAppear): string {
        // pc 일 경우 group안에 entity가 여러개 생기지 않도록
        if (this.pcRaceSet.has(event.RaceId)) {
            return event.Id;
        }

        return `${event.RaceId}`;
    }
}

interface IEventActor {
    /** damage 쪽 수치들만 reset */
    clear(): void;
}

export abstract class BaseActor implements IEventActor {
    protected constructor(protected mgr: ActorManager, private _id: string, private _raceId: number, protected _name: string) {
        this._isPC = ActorManager.pcRaceSet.has(_raceId);
    }

    public get id() {
        return this._id;
    }

    public get raceId() {
        return this._raceId;
    }

    public get name() {
        return this._name;
    }

    protected _body: EntityBody = shallowReactive({
        Height: 1,
        Weight: 1,
        Upper: 1,
        Lower: 1,
    });
    public get body() {
        return this._body;
    }

    /** 받은 대미지 */
    public get totalTakeDamage() {
        return this._totalTakeDamage;
    }
    protected _totalTakeDamage = 0;

    protected _takeDamages: EntityDamage[] = [];
    public get takeDamages() {
        return this._takeDamages;
    }

    /** 준 대미지 */
    public get totalApplyDamage() {
        return this._totalApplyDamage;
    }
    protected _totalApplyDamage = 0;

    protected _applyDamages: EntityDamage[] = [];
    public get applyDamages() {
        return this._applyDamages;
    }

    private _isPC = false;
    public get isPC() {
        return this._isPC;
    }

    public onEntityAppear(event: protocols.eventEntityAppear): void {
        // nothing
        event;
    }

    public onTakeDamage(event: protocols.eventDamage): void {
        // nothing
        event;
    }

    public onApplyDamage(event: protocols.eventDamage): void {
        // nothing
        event;
    }

    public onCharacterConditionEnable(event: protocols.eventCharacterConditionEnable): void {
        // nothing
        event;
    }

    public onCharacterConditionDisable(event: protocols.eventCharacterConditionDisable): void {
        // nothing
        event;
    }

    public onFinish(event: protocols.eventFinish): void {
        // nothing
        event;
    }

    public onEquipItem(event: protocols.eventEntityEquipItem): void {
        // nothing
        event;
    }

    public onUnequipItem(event: protocols.eventEntityUnequipItem): void {
        // nothing
        event;
    }

    public onUpdateBody(event: protocols.eventEntityUpdateBody): void {
        this._body.Height = event.Height;
        this._body.Weight = event.Weight;
        this._body.Upper = event.Upper;
        this._body.Lower = event.Lower;
    }

    public clear() {
        this._totalTakeDamage = 0;
        this._takeDamages.length = 0;

        this._totalApplyDamage = 0;
        this._applyDamages.length = 0;
    }
}

export class EntityActor extends BaseActor {
    public constructor(mgr: ActorManager, id: string, raceId: number, name: string, private _group: GroupActor) {
        super(mgr, id, raceId, name);
    }

    protected _guildName = '';
    public get guildName() {
        return this._guildName;
    }

    protected _ownerId = '';
    public get ownerId() {
        return this._ownerId;
    }

    public get group() {
        return this._group;
    }

    protected _conditionMap: Record<number, EntityCondition> = shallowReactive({});
    public get conditionMap() {
        return this._conditionMap;
    }

    protected _conditionHistory: EntityConditionState[] = [];

    private _finisherId = '';
    public get finisherId() {
        return this._finisherId;
    }

    protected _equipItemMap: Record<number, EntityItem> = shallowReactive({});
    public get equipItemMap() {
        return this._equipItemMap;
    }

    public override onEntityAppear(event: protocols.eventEntityAppear): void {
        this._finisherId = '';
        this._guildName = event.GuildName;
        this._ownerId = event.OwnerId;
        this._body.Height = event.Height;
        this._body.Weight = event.Weight;
        this._body.Upper = event.Upper;
        this._body.Lower = event.Lower;

        if (ActorManager.pcRaceSet.has(event.RaceId)) {
            // pc일 경우 damage 초기와 안함
            return;
        }

        this._totalTakeDamage = 0;
        this._takeDamages.length = 0;
    }

    public override onTakeDamage(event: protocols.eventDamage): void {
        const attacker = this.mgr.entityMap[event.Id];

        const damage: EntityDamage = {
            ...event,

            Conditions: attacker?.getConditionState(event.At) ?? [],
            TargetConditions: this.getConditionState(event.At),
        }

        this._totalTakeDamage += event.Damage;
        this._takeDamages.push(damage);
    }

    public override onApplyDamage(event: protocols.eventDamage): void {
        const targetId = event.TargetId;
        const target = this.mgr.entityMap[targetId];
        if (!target || !(target instanceof EntityActor)) {
            // 몹 정보가 없으면 무시
            return;
        }

        const damage: EntityDamage = {
            ...event,

            Conditions: this.getConditionState(event.At),
            TargetConditions: target.getConditionState(event.At),
        }

        this._totalApplyDamage += event.Damage;
        this._applyDamages.push(damage);

        // apply에서만 호출
        this.mgr.onEntityDamage(damage);
    }

    public override onCharacterConditionEnable(event: protocols.eventCharacterConditionEnable): void {
        this._conditionMap[event.CCId] = {
            Id: event.Id,
            At: event.At,
            CCId: event.CCId,
            DisableAt: event.DisableAt,
            AttackerId: event.AttackerId,
        };

        const prev = this._conditionHistory.length ? this._conditionHistory[this._conditionHistory.length - 1].List : [];
        const current = Object.values(this._conditionMap).sort((a, b) => a.CCId - b.CCId);

        const needUpdate = prev.length !== current.length || !prev.every((v, i) => v.CCId === current[i].CCId);
        if (needUpdate) {
            this._conditionHistory.push({
                At: event.At,
                List: current,
            });
        }
    }

    public override onCharacterConditionDisable(event: protocols.eventCharacterConditionDisable): void {
        delete this._conditionMap[event.CCId];

        const prev = this._conditionHistory.length ? this._conditionHistory[this._conditionHistory.length - 1].List : [];
        const current = Object.values(this._conditionMap).sort((a, b) => a.CCId - b.CCId);

        const needUpdate = prev.length !== current.length || !prev.every((v, i) => v.CCId === current[i].CCId);
        if (needUpdate) {
            this._conditionHistory.push({
                At: event.At,
                List: current,
            });
        }
    }

    public override onFinish(event: protocols.eventFinish): void {
        this._finisherId = event.AttackerId;
    }

    public override onEquipItem(event: protocols.eventEntityEquipItem): void {
        // this._equipItemMap[event.PocketType] = {
        //     ...event,
        // };
        this._equipItemMap[event.PocketType] = event;
    }

    public override onUnequipItem(event: protocols.eventEntityUnequipItem): void {
        delete this._equipItemMap[event.PocketType];
    }

    public getConditionState(at: number): EntityCondition[] {
        const idx = bounds.le<{ At: number }>(this._conditionHistory, { At: at }, (a, b) => a.At - b.At);
        if (idx < 0) {
            return [];
        }

        return this._conditionHistory[idx].List;
    }

    public override clear() {
        super.clear();
    }
}

// TODO: GroupActor에 Group 조건 추가하는 식으로 바꾸는게 좋을듯
export class GroupActor extends BaseActor {
    public constructor(mgr: ActorManager, id: string, raceId: number, name: string) {
        const groupName = ActorManager.pcRaceSet.has(raceId)
            ? name : `${raceId}`;

        super(mgr, id, raceId, groupName);
    }

    private _entityMap: Record<string, EntityActor> = shallowReactive({});
    public get entityMap() {
        return this._entityMap;
    }

    public override onEntityAppear(event: protocols.eventEntityAppear): void {
        if (ActorManager.pcRaceSet.has(event.RaceId)) {
            // pc일 경우 damage 초기와 안함
            return;
        }

        const target = this._entityMap[event.Id];
        if (!target) {
            // ?
            return;
        }

        this._takeDamages = this._takeDamages.filter(v => v.Id !== event.Id);
        this._totalTakeDamage = this._takeDamages.reduce((acc, v) => acc + v.Damage, 0);
    }

    public override onTakeDamage(event: protocols.eventDamage): void {
        // console.log('group take damage', this.id, event);

        const attacker = this.mgr.entityMap[event.Id];
        const target = this._entityMap[event.TargetId];
        if (!target) {
            // ?
            return;
        }

        const damage: EntityDamage = {
            ...event,

            Conditions: attacker?.getConditionState(event.At) ?? [],
            TargetConditions: target.getConditionState(event.At),
        }

        this._totalTakeDamage += event.Damage;
        this._takeDamages.push(damage);
    }

    public override clear(): void {
        super.clear();
    }
}

export type EntityDamage = {
    Id: string;
    At: number;
    TargetId: string;
    SkillId: number;
    Damage: number;
    IsCritical: boolean;
    Conditions: EntityCondition[];
    TargetConditions: EntityCondition[];
};

export type EntityCondition = {
    Id: string;
    At: number;
    CCId: number;
    DisableAt: number;
    AttackerId: string;
}

type EntityConditionState = {
    At: number;
    List: EntityCondition[];
}

export type EntityItem = {
    PocketType: number;
    ItemId: number;
    Color1: string;
    Color2: string;
    Color3: string;
    Color5: string;
    Color6: string;
    Color7: string;
}

export type EntityBody = {
    Height: number;
    Weight: number;
    Upper: number;
    Lower: number;
}