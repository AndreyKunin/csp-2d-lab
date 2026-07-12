import * as math from 'mathjs';
import {Fraction} from "mathjs";

export const getFillPattern = (value: string): string => {
    return value.replace(/\d/g, '8').replace(/\D/g, ' ');
}

export const isFraction = (value: string): boolean => {
    return value.includes('/');
};

export const parseSize = (sizeStr: string, fractionExpected: boolean): Fraction | number => {
    const fractionActual = isFraction(sizeStr);
    if (fractionActual) {
        const parts = sizeStr.split('/').map(s => s.trim());
        if (parts[0].includes(' ')) {
            const subParts = parts[0].split(' ')
                .filter(s => s.length > 0)
                .map(s => s.trim());
            const fractionValue = math.add(math.fraction(subParts[0]), math.fraction(`${subParts[1]}/${parts[1]}`));
            return fractionExpected ? fractionValue : math.number(fractionValue);
        }
        const fractionValue = math.fraction(sizeStr.trim());
        return fractionExpected ? fractionValue : math.number(fractionValue);
    }
    const numberValue = math.number(sizeStr);
    return fractionExpected ? math.fraction(numberValue) : numberValue;
};

export const formatSize = (size: Fraction | number, fractionExpected: boolean): string => {
    if (fractionExpected) {
        const whole = math.floor(size);
        if (math.larger(whole, 0)) {
            const fractionPart = math.subtract(size, whole);
            if (math.equal(math.number(0), fractionPart)) {
                return math.format(whole, {fraction: 'decimal', notation: 'fixed'});
            } else {
                return `${math.format(whole, {
                    fraction: 'decimal',
                    notation: 'fixed'
                })} ${math.format(fractionPart, {fraction: 'ratio', notation: 'fixed'})}`;
            }
        }
        return math.format(size, {fraction: 'ratio', notation: 'fixed'});
    }
    return math.format(size, {fraction: 'decimal', notation: 'fixed'});
};