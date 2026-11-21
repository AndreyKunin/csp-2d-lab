import {SimulationParameters, SimulationRuntimeParameters} from "./simulation";
import {MutableRefObject} from "react";
import {isFraction} from "./numbers";

export interface PropertyItem {
    index: number;
    key: string;
    displayName: string;
    value: string;
    tooltip?: string;
}


export function getInitialParameters(defaultParameters: SimulationParameters): PropertyItem[] {
    const populationSizeTooltip = 'Population size a number of solutions within the population, which is maintained across generations. This value is adaptive and depends on the problem size. However, we set boundaries for minimum and maximum values to reduce compute requirements.';
    return  [
        {
            index: 0, key: 'populationSize.min',
            displayName: 'Minimum population size',
            value: defaultParameters.populationSize?.min.toString() || '',
            tooltip: populationSizeTooltip,
        },
        {
            index: 1, key: 'populationSize.max',
            displayName: 'Maximum population size',
            value: defaultParameters.populationSize?.max.toString() || '',
            tooltip: populationSizeTooltip,
        },
        {
            index: 2, key: 'eliteCount',
            displayName: 'Elite count',
            value: defaultParameters.eliteCount?.toString() || '',
            tooltip: 'The elite size specifies the number of the fittest individuals that are guaranteed to be carried over from one generation to the next without any changes.',
        },
        {
            index: 3, key: 'mutationRate',
            displayName: 'Mutation rate',
            value: defaultParameters.mutationRate?.toString() || '',
            tooltip: 'Mutation rate is the probability that a single element of the solution will be altered during the creation of a new offspring solution.',
        },
        {
            index: 4, key: 'crossoverRate',
            displayName: 'Crossover rate',
            value: defaultParameters.crossoverRate?.toString() || '',
            tooltip: 'Share of the non-elite offspring made by crossover, from 0 (exclusive) to 1. The rest are copies of tournament-selected individuals, changed by mutation only.',
        },
        {
            index: 5, key: 'minRate',
            displayName: 'Minimum rate allowed',
            value: defaultParameters.minRate?.toString() || '',
            tooltip: 'Lower bound of the Crossover and Mutation Rate. Some of problems tend to raise the exploitation undefinitely that leads to convergence to local optima. This parameter prevents that.',
        },
        {
            index: 6, key: 'maxRate',
            displayName: 'Maximum rate allowed',
            value: defaultParameters.maxRate?.toString() || '',
            tooltip: 'Upper bound of the Crossover and Mutation Rate. Some of problems tend to raise the exploration undefinitely that leads to random search. This parameter prevents that.',
        },
        {
            index: 7, key: 'maxEpochsCount',
            displayName: 'Number of generations',
            value: defaultParameters.maxEpochsCount?.toString() || '',
            tooltip: 'Maximum nimber of epochs (generations) in the simulation. Simulation is a process that never halts until manually stopped. We can auto-stop the process when it reaches the maximum number of epochs.',
        },
        {
            index: 8, key: 'windowSize',
            displayName: 'Window size',
            value: defaultParameters.windowSize?.toString() || '',
            tooltip: 'Number of epochs without improvement of the best solution that ends a cycle (and the championship). Also the length of the sliding window that smooths the charted metrics.',
        },
        {
            index: 9, key: 'cycleCount',
            displayName: 'Cycles before championship',
            value: defaultParameters.cycleCount?.toString() || '',
            tooltip: 'When the best solution stops improving for a window, the cycle ends: its elite is kept and the simulation starts over from random solutions. After this many cycles, all kept elites compete in a final championship; the simulation ends when it stops improving too. 0 = end at the first stagnation.',
        }
    ];
}

export function getRuntimeParameters(runtimeParameters?: SimulationRuntimeParameters): PropertyItem[] {
    if (!runtimeParameters) {
        return [];
    }
    return  [
        {
            index: 0, key: 'eliteCount',
            displayName: 'Elite count',
            value: runtimeParameters.eliteCount.toString() || '',
        },
        {
            index: 1, key: 'mutationRate',
            displayName: 'Mutation rate',
            value: runtimeParameters.mutationRate.toString() || '',
        },
        {
            index: 2, key: 'crossoverRate',
            displayName: 'Crossover rate',
            value: runtimeParameters.crossoverRate.toString() || '',
        },
        {
            index: 3, key: 'minRate',
            displayName: 'Minimum rate allowed',
            value: runtimeParameters.minRate.toString() || '',
        },
        {
            index: 4, key: 'maxRate',
            displayName: 'Maximum rate allowed',
            value: runtimeParameters.maxRate.toString() || '',
        },
        {
            index: 5, key: 'maxEpochsCount',
            displayName: 'Number of generations',
            value: runtimeParameters.maxEpochsCount.toString() || '',
        },
        {
            index: 6, key: 'windowSize',
            displayName: 'Window size',
            value: runtimeParameters.windowSize.toString() || '',
        },
        {
            index: 7, key: 'cycleCount',
            displayName: 'Cycles before championship',
            value: (runtimeParameters.cycleCount ?? 0).toString(),
        }
    ];
}

export function getSimulationParameters(items: PropertyItem[]) {
    const propertyMap = new Map(
        items.map(item => [item.key, item.value])
    );
    const parameters: SimulationParameters = {
        eliteCount: Number(propertyMap.get('eliteCount')),
        mutationRate: Number(propertyMap.get('mutationRate')),
        crossoverRate: Number(propertyMap.get('crossoverRate')),
        minRate: Number(propertyMap.get('minRate')),
        maxRate: Number(propertyMap.get('maxRate')),
        maxEpochsCount: Number(propertyMap.get('maxEpochsCount')),
        windowSize: Number(propertyMap.get('windowSize')),
        cycleCount: Number(propertyMap.get('cycleCount')),
    };
    if (propertyMap.has('populationSize.min')) {
        parameters.populationSize = {
            min: Number(propertyMap.get('populationSize.min')),
            max: Number(propertyMap.get('populationSize.max'))
        };
    }
    return parameters;
}

export function validateSimulationParameters(
    allowEmpty: boolean, key: string, value: string, errorsRef: MutableRefObject<Map<string, boolean>>
) {
    if (!allowEmpty && !value) {
        setPropertyInvalid(key, true, errorsRef);
        return false;
    }
    let valid = false;
    if (key.includes('Size') || key.includes('Count')) {
        valid = validateCount(value);
    } else if (key.includes('Rate') || key.includes('Delta') || key.includes('Probability')) {
        valid = validateDecimal(value);
    }

    if (!valid) {
        setPropertyInvalid(key, true, errorsRef);
    } else {
        setPropertyInvalid(key, false, errorsRef);
    }
    return valid;
}

export function setPropertyInvalid (
    key: string, value: boolean, errorsRef: MutableRefObject<Map<string, boolean>>
){
    errorsRef.current.set(key, value);
}

export function validateSize (value: string): boolean {
    const hasValue = !!value;
    const fraction = isFraction(value);
    return !hasValue || (fraction && validateFraction(value) || !fraction && !isNaN(Number(value)) && Number(value) > 0);
}

export function validateCount (value: string): boolean {
    const hasValue = !!value;
    return !hasValue || validateInteger(value);
}

export function validateInteger (value: string): boolean {
    const pattern = /^(\d+)$/;
    return pattern.test(value.trim());
}

export function validateDecimal (value: string): boolean {
    const pattern = /^(\.?[0-9]+|[0-9]+\.?[0-9]+|[0-9]+)$/;
    return pattern.test(value.trim());
}

export function validateFraction (value: string): boolean {
    const pattern = /^(\d+|\d+ *\/ *\d+|\d+ *\d+ *\/ *\d+)$/;
    return pattern.test(value.trim());
}