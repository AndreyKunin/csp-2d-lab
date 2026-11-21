import {parseSize, isFraction, formatSize} from './numbers';
import {InputFormData} from "../components/InputForm";
import * as math from 'mathjs';
import {Fraction} from "mathjs";

export interface Part {
    label: string;
    width: string;
    length: string;
    count: string;
}

export interface Sheet {
    width: string;
    length: string;
}

export interface Solution {
    sheets: SheetData[];
    fitness?: number;
    fitnessVector?: number[];
}

export interface SheetData {
    number: number;
    tree: string;
}

export class Model {
    units: string;
    sheets?: Sheet;
    parts: Part[];
    fraction: boolean;
    sawWidth: string;
    solutions?: Solution[];

    constructor(formData: InputFormData) {
        const {unitType, partsData, sheetsData, sawWidth} = formData;
        let fraction = false;

        this.units = unitType;
        this.parts = [];
        this.fraction = false;
        this.sawWidth = sawWidth;

        if (sheetsData) {
            let sheet: Sheet = {
                width: sheetsData[0][0],
                length: sheetsData[0][1]
            };
            this.sheets = sheet;
            if (!fraction && (isFraction(sheet.width) || isFraction(sheet.length))) {
                fraction = true;
            }
        }

        for (let i = 0; i < partsData.length; ++i) {
            let part: Part = {
                label: `${i + 1}`,
                width: partsData[i][0],
                length: partsData[i][1],
                count: partsData[i][2]
            };
            this.parts.push(part);
            if (!fraction && (isFraction(part.width) || isFraction(part.length) || isFraction(part.count))) {
                fraction = true;
            }
        }

        this.fraction = fraction;
    }


    public validate(): string[] {
        const MIN_TOTAL_COUNT = 2;
        const MAX_TOTAL_COUNT = 100; // hard limit on the total part count (sum of the counts); not configurable
        const MAX_MAGNITUDE = 4;
        let sheetW: Fraction | number = 0, sheetL: Fraction | number = 0;
        let totalCount: number = 0, maxValue: Fraction | number = 0,
            minValue: Fraction | number = Number.MAX_SAFE_INTEGER;
        let errors: string[] = [], duplicates = new Map();

        const numericSheets = this.getNumericSheets(false);
        const numericParts = this.getNumericParts(false);

        if (!numericParts.length) {
            errors.push('Please specify parts.');
            return errors;
        }

        if (numericSheets) {
            sheetW = numericSheets.width;
            sheetL = numericSheets.length;
            maxValue = this.getMax(sheetW, sheetL, maxValue);
            minValue = this.getMin(sheetW, sheetL, minValue);
        }

        for (let i = 0; i < numericParts.length; ++i) {
            const {width, length, count} = numericParts[i];
            this.checkDuplicate(i, width, length, duplicates, this.fraction, errors);
            this.checkValue(i, width, this.fraction, errors);
            this.checkValue(i, length, this.fraction, errors);
            if (numericSheets) {
                this.checkSheetFitsSize(i, width, length, sheetW, sheetL, this.fraction, errors);
            }
            maxValue = this.getMax(width, length, maxValue);
            minValue = this.getMin(width, length, minValue);
            totalCount += math.number(count);
        }

        if (totalCount < MIN_TOTAL_COUNT) {
            errors.push(`Total number of parts less than ${MIN_TOTAL_COUNT} does not make sense in this problem.`);
        }
        if (totalCount > MAX_TOTAL_COUNT) {
            errors.push(`Total number of parts is limited to ${MAX_TOTAL_COUNT}. Requested: ${totalCount}. Please reduce the number of parts.`);
        }
        const q = math.divide(maxValue, minValue);
        if (math.smaller(math.number(MAX_MAGNITUDE), math.log10(math.number(q as Fraction | number)))) {
            errors.push(`The highest size ${formatSize(maxValue, this.fraction)} is more than ${MAX_MAGNITUDE} orders of magnitude greater than the smallest size ${formatSize(minValue, this.fraction)}. The computation method couldn't produce any good result from such values. Please make sizes closer to each other or compute them separately.`);
        }

        return errors;
    }

    public getNumericSheets(asNumber: boolean): {
        width: Fraction | number;
        length: Fraction | number;
    } | undefined {
        if (this.sheets) {
            return {
                width: this.getNumber(this.sheets.width, asNumber),
                length: this.getNumber(this.sheets.length, asNumber)
            };
        } else {
            return undefined;
        }
    }

    public getNumericParts(asNumber: boolean): {
        label: string,
        width: Fraction | number;
        length: Fraction | number;
        count: Fraction | number;
    }[] {
        let numericParts = [];

        for (let i = 0; i < this.parts.length; ++i) {
            numericParts.push({
                label: this.parts[i].label,
                width: this.getNumber(this.parts[i].width, asNumber),
                length: this.getNumber(this.parts[i].length, asNumber),
                count: this.getNumber(this.parts[i].count, asNumber)
            });
        }

        return numericParts;
    }

    public getNumber(value: string,  asNumber: boolean): Fraction | number {
        const numberValue = parseSize(value, this.fraction);
        return asNumber ? math.number(numberValue) : numberValue;
    }

    // Helper functions
    private getMin(
        value1: Fraction | number, value2: Fraction | number, minValue?: Fraction | number
    ): Fraction | number {
        return minValue ? math.min(value1, value2, minValue) : math.min(value1, value2);
    }

    private getMax(
        value1: Fraction | number, value2: Fraction | number, maxValue?: Fraction | number
    ): Fraction | number {
        return maxValue ? math.max(value1, value2, maxValue) : math.max(value1, value2);
    }

    private checkSheetFitsSize(index: number,
                               width: Fraction | number, height: Fraction | number,
                               maxW: Fraction | number, maxH: Fraction | number,
                               fraction: boolean, errors: string[]) {
        const mW = math.max(maxW, maxH);
        const mH = math.min(maxW, maxH);
        const w = math.max(width, height);
        const h = math.min(width, height);
        if (math.larger(w, mW) || math.larger(w, mH) && math.larger(h, mH)) {
            const sizeStr = `${formatSize(maxW, fraction)} x ${formatSize(maxH, fraction)}`;
            const dimStr = `${formatSize(width, fraction)} x ${formatSize(height, fraction)}`;
            errors.push(`The part # ${index + 1} (${dimStr}) does not fit the given sheet size (${sizeStr})`);
        }
    }

    private checkDuplicate(index: number,
                           width: Fraction | number, height: Fraction | number,
                           duplicates: Map<string, number>, fraction: boolean, errors: string[]) {
        const w = math.max(width, height);
        const h = math.min(width, height);
        const dimStr = `${formatSize(w, fraction)} x ${formatSize(h, fraction)}`;
        if (duplicates.has(dimStr)) {
            errors.push(`The parts # ${duplicates.get(dimStr)! + 1} and # ${index + 1} (${dimStr}) are duplicates.`);
        }
        duplicates.set(dimStr, index);
    }

    private checkValue(index: number, value: Fraction | number, fraction: boolean, errors: string[]) {
        const MAX_PRECISION = 1000;
        if (fraction) {
            const v = value as Fraction
            let denominator = v.d;
            if (denominator > MAX_PRECISION) {
                errors.push(`The part # ${index + 1} has precision of 1/${denominator}. Max supported precision is 1/${MAX_PRECISION}.`);
            }
        } else {
            let ONE = math.number(1);
            let MAX_PRECISION_NUM = math.number(MAX_PRECISION);
            let precision: Fraction | number = math.mod(value, ONE);
            if (math.larger(precision, 0) && math.smaller(math.multiply(precision, MAX_PRECISION_NUM), ONE)) {
                errors.push(`The part # ${index + 1} has precision of ${formatSize(math.fraction(precision), true)}. Max supported precision is 1/${MAX_PRECISION}.`);
            }
        }
    }
}
