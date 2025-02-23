import { reactive } from 'vue';

import { EntityDamage } from '@/eventActor';

// TODO: cc 추가
export class DamageCollectorManager {
    public get damages() {
        return this._damages;
    }
    private _damages = [] as EntityDamage[];

    // 성능 보고 entity id별로 쪼갤지 확인
    private _et = new DamageEventTarget("Damage", "DamageCollector");

    public onDamage(p: EntityDamage): void {
        this._damages.push(p);
        this._et.dispatchEvent(new CustomEvent("Damage", { detail: p }));
    }

    public getFilteredDamageCollector(filter: DamageCollectorFilter): FilteredDamageCollector {
        const v = new FilteredDamageCollector(filter);
        for (const p of this._damages) {
            v.handleDamage(p);
        }

        // et -> filtered et -> filtered et로 chaining하는게 나을듯
        this._et.addEventListener("Damage", v);
        this._et.addEventListener("Clear", v);

        return v;
    }

    public getGroupedDamageCollector(filter: DamageCollectorFilter, getGroupKey: DamageCollectorGroupKey): GroupedDamageCollector {
        const v = new GroupedDamageCollector(filter, getGroupKey);
        for (const p of this._damages) {
            v.handleDamage(p);
        }

        this._et.addEventListener("Damage", v);
        this._et.addEventListener("Clear", v);

        return v;
    }

    public getDualGroupedDamageCollector(filter: DamageCollectorFilter, getGroupKey1: DamageCollectorGroupKey, getGroupKey2: DamageCollectorGroupKey): DualGroupedDamageCollector {
        const v = new DualGroupedDamageCollector(filter, getGroupKey1, getGroupKey2);
        for (const p of this._damages) {
            v.handleDamage(p);
        }

        this._et.addEventListener("Damage", v);
        this._et.addEventListener("Clear", v);

        return v;
    }

    public removeDamageCollector(collector: DamageCollectorBase): void {
        this._et.removeEventListener("Damage", collector);
        this._et.removeEventListener("Clear", collector);
    }

    public clear(): void {
        this._damages = [];
        this._et.dispatchEvent(new CustomEvent("Clear"));
    }
}

export abstract class DamageCollectorBase implements DamageEventListenerObject {
    public constructor(private _filter: DamageCollectorFilter) {
    }

    public handleEvent(e: CustomEvent<EntityDamage>): void {
        switch (e.type) {
            case "Damage":
                this.handleDamage(e.detail);
                break;

            case "Clear":
                this.onClear();
                break;
        }
    }

    public handleDamage(p: EntityDamage): void {
        if (!this._filter(p)) {
            return;
        }

        this.onDamage(p);
    }

    protected abstract onDamage(p: EntityDamage): void;

    protected abstract onClear(): void;
}

export class FilteredDamageCollector extends DamageCollectorBase {
    public constructor(filter: DamageCollectorFilter) {
        super(filter);
    }

    public get damages() {
        return this._damages;
    }
    private _damages = [] as EntityDamage[];

    public get totalDamage() {
        return this._totalDamage.value;
    }
    private _totalDamage = reactive({value: 0});

    protected override onDamage(p: EntityDamage): void {
        this._damages.push(p);
        this._totalDamage.value += p.Damage;
    }

    protected override onClear(): void {
        this._damages.length = 0;
        this._totalDamage.value = 0;
    }
}

export class GroupedDamageCollector extends FilteredDamageCollector {
    public constructor(filter: DamageCollectorFilter, protected _getGroupKey: DamageCollectorGroupKey) {
        super(filter);
    }

    private _groupedDamages: Record<string, EntityDamage[]> = {};
    public get groupedDamages() {
        return this._groupedDamages;
    }

    private _groupedTotalDamages: Record<string, number> = reactive({});
    public get groupedTotalDamages() {
        return this._groupedTotalDamages;
    }

    protected override onDamage(p: EntityDamage): void {
        super.onDamage(p);

        const key = this._getGroupKey(p);
        if (!this._groupedDamages[key]) {
            this._groupedDamages[key] = [];
            this._groupedTotalDamages[key] = 0;
        }

        this._groupedDamages[key].push(p);
        this._groupedTotalDamages[key] += p.Damage;
    }

    protected override onClear(): void {
        super.onClear();

        for (const k in this._groupedDamages) {
            delete this._groupedDamages[k];
        }

        for (const k in this._groupedTotalDamages) {
            delete this._groupedTotalDamages[k];
        }
    }
}

export class DualGroupedDamageCollector extends GroupedDamageCollector {
    public constructor(filter: DamageCollectorFilter, getGroupKey1: DamageCollectorGroupKey, private _getGroupKey2: DamageCollectorGroupKey) {
        super(filter, getGroupKey1);
    }

    private _grouped2Damages: Record<string, EntityDamage[]> = {};
    public get grouped2Damages() {
        return this._grouped2Damages;
    }

    private _grouped2TotalDamages: Record<string, number> = reactive({});
    public get grouped2TotalDamages() {
        return this._grouped2TotalDamages;
    }

    private _dualGroupedDamages: Record<string, Record<string, EntityDamage[]>> = {};
    public get dualGroupedDamages() {
        return this._dualGroupedDamages;
    }

    private _dualGroupedTotalDamages: Record<string, Record<string, number>> = reactive({});
    public get dualGroupedTotalDamages() {
        return this._dualGroupedTotalDamages;
    }

    protected override onDamage(p: EntityDamage) {
        super.onDamage(p);

        const key1 = this._getGroupKey(p);
        const key2 = this._getGroupKey2(p);

        if (!this._grouped2Damages[key2]) {
            this._grouped2Damages[key2] = [];
            this._grouped2TotalDamages[key2] = 0;
        }

        this._grouped2Damages[key2].push(p);
        this._grouped2TotalDamages[key2] += p.Damage;

        if (!this._dualGroupedDamages[key1]) {
            this._dualGroupedDamages[key1] = {};
            this._dualGroupedTotalDamages[key1] = {};
        }

        if (!this._dualGroupedDamages[key1][key2]) {
            this._dualGroupedDamages[key1][key2] = [];
            this._dualGroupedTotalDamages[key1][key2] = 0;
        }

        this._dualGroupedDamages[key1][key2].push(p);
        this._dualGroupedTotalDamages[key1][key2] += p.Damage;
    }

    protected override onClear() {
        super.onClear();

        for (const k in this._grouped2Damages) {
            delete this._grouped2Damages[k];
        }

        for (const k in this._grouped2TotalDamages) {
            delete this._grouped2TotalDamages[k];
        }

        for (const k in this._dualGroupedDamages) {
            delete this._dualGroupedDamages[k];
        }

        for (const k in this._dualGroupedTotalDamages) {
            delete this._dualGroupedTotalDamages[k];
        }
    }
}

/*
export class DamageCollectorFilter {
    public get filter() {
        return this._filter;
    }

    public constructor(private _filter: (p: EntityDamage) => boolean) {
    }

    public check(p: EntityDamage) {
        return this._filter(p);
    }
}
*/

type DamageCollectorFilter = (p: EntityDamage) => boolean;
type DamageCollectorGroupKey = (p: EntityDamage) => string;

// type DamageEventType = "TakeDamage" | "ApplyDamage";
type DamageEventType = "Damage" | "Clear";

interface IDamageEventTarget extends EventTarget {
    addEventListener(type: DamageEventType, callback: DamageEventListenerObject, options?: boolean | AddEventListenerOptions): void
    removeEventListener(type: DamageEventType, callback: DamageEventListenerObject, options?: boolean | EventListenerOptions): void;
}

/*
interface DamageEventListener extends EventListener {
    (evt: CustomEvent<EntityDamage>): void;
}
*/

interface DamageEventListenerObject extends EventListenerObject {
    handleEvent(evt: CustomEvent<EntityDamage>): void;
}

class DamageEventTarget extends EventTarget implements IDamageEventTarget {
    public get type() {
        return this._type;
    }

    public get id() {
        return this._id;
    }

    public constructor(private _type: DamageEventType, private _id: string) {
        super();
    }

    public get count() {
        return this._count;
    }
    private _count = 0;

    private cbSet: Record<string, Set<DamageEventListenerObject>> = {};

    public override addEventListener(type: DamageEventType, listener: DamageEventListenerObject, options?: boolean | AddEventListenerOptions): void {
        if (this.cbSet[type]?.has(listener)) {
            return;
        }

        if (!this.cbSet[type]) {
            this.cbSet[type] = new Set();
        }

        this.cbSet[type].add(listener);
        super.addEventListener(type, listener, options);
        this._count++;
    }

    public override removeEventListener(type: DamageEventType, listener: DamageEventListenerObject, options?: boolean | EventListenerOptions): void {
        if (!this.cbSet[type]?.has(listener)) {
            return;
        }

        if (!this.cbSet[type]) {
            // ?
            return;
        }

        this.cbSet[type].delete(listener);
        super.removeEventListener(type, listener, options);
        this._count--;
    }
}
