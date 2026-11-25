import {SimulationParameters, SimulationRuntimeParameters} from "./simulation";
import {PARAMETER_DEFINITIONS, ParameterDefinition, findParameter, validateParameter, valueOf} from "./parameterDefinitions";
import {MutableRefObject} from "react";
import {isFraction} from "./numbers";

export interface PropertyItem {
    index: number;
    key: string;
    displayName: string;
    value: string;
    tooltip?: string;
    disabled?: boolean; // overridden by the server: shown empty, not editable, not sent
}


function toItems(definitions: ParameterDefinition[], source: object | undefined): PropertyItem[] {
    return definitions.map((definition, index) => {
        const value = valueOf(source, definition.key);
        return {
            index,
            key: definition.key,
            displayName: definition.displayName,
            value: value === undefined || value === null ? '' : String(value),
            tooltip: definition.tooltip,
        };
    });
}

// The start form's custom parameters: every defined parameter, valued from the
// server defaults.
export function getInitialParameters(defaultParameters: SimulationParameters): PropertyItem[] {
    return toItems(PARAMETER_DEFINITIONS, defaultParameters);
}

// The Lab Edit dialog's parameters: the runtime ones, valued from the running
// job's parameters.
export function getRuntimeParameters(runtimeParameters?: SimulationRuntimeParameters): PropertyItem[] {
    if (!runtimeParameters) {
        return [];
    }
    return toItems(PARAMETER_DEFINITIONS.filter(d => d.runtime), runtimeParameters);
}

// getSimulationParameters builds the request parameters from the items: every
// item becomes a number; a dotted key becomes a nested field.
export function getSimulationParameters(items: PropertyItem[]): SimulationParameters {
    const parameters: Record<string, unknown> = {};
    for (const item of items) {
        const path = item.key.split('.');
        let target = parameters;
        for (const part of path.slice(0, -1)) {
            if (typeof target[part] !== 'object' || target[part] === null) {
                target[part] = {};
            }
            target = target[part] as Record<string, unknown>;
        }
        target[path[path.length - 1]] = Number(item.value);
    }
    return parameters as SimulationParameters;
}

// validateRuntimeParameters checks the Edit dialog's values: every value against
// its rule, then (when all are valid) against the running job, whose population
// size and window size the dialog cannot change: the elite count is below the
// population size (the server's rule, ValidateBuiltParameters), and the number of
// generations is at least the window size (as the start form checks). It returns
// the messages for the user, none when the values are valid (FE-02).
export function validateRuntimeParameters(items: PropertyItem[], running?: SimulationRuntimeParameters): string[] {
    const messages: string[] = [];
    for (const item of items) {
        const error = validateParameter(item.key, item.value);
        if (error) {
            messages.push(error);
        }
    }
    if (messages.length > 0 || !running) {
        return messages;
    }
    const value = (key: string) => {
        const item = items.find(i => i.key === key);
        return item ? Number(item.value) : undefined;
    };
    const eliteCount = value('eliteCount');
    if (eliteCount !== undefined && eliteCount >= running.populationSize) {
        messages.push(`Elite count must be less than the population size (${running.populationSize}).`);
    }
    const maxEpochsCount = value('maxEpochsCount');
    if (maxEpochsCount !== undefined && maxEpochsCount < running.windowSize) {
        messages.push(`Number of generations must be at least the window size (${running.windowSize}).`);
    }
    return messages;
}

// submitRuntimeParameters validates the Edit dialog's values and, when they are
// valid, submits them (configure). It returns the messages to show: the
// validation errors, or the error of the submit; none when it succeeded.
export async function submitRuntimeParameters(
    items: PropertyItem[], running: SimulationRuntimeParameters | undefined,
    submit: (parameters: SimulationParameters) => Promise<void>
): Promise<string[]> {
    const messages = validateRuntimeParameters(items, running);
    if (messages.length > 0) {
        return messages;
    }
    try {
        await submit(getSimulationParameters(items));
        return [];
    } catch (error) {
        return [error instanceof Error ? error.message : String(error)];
    }
}

// validateSimulationParameters checks one parameter value against its rule
// (the server's rules, parameterDefinitions.ts) and records the result for the grid. allowEmpty
// accepts an empty value (while the user types); a submit passes false.
export function validateSimulationParameters(
    allowEmpty: boolean, key: string, value: string, errorsRef: MutableRefObject<Map<string, boolean>>
) {
    let valid: boolean;
    if (!value) {
        valid = allowEmpty;
    } else {
        const error = validateParameter(key, value);
        if (error && !findParameter(key)) {
            console.error(error); // a parameter without a definition: a programming error
        }
        valid = error === null;
    }
    setPropertyInvalid(key, !valid, errorsRef);
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

export function validateFraction (value: string): boolean {
    const pattern = /^(\d+|\d+ *\/ *\d+|\d+ *\d+ *\/ *\d+)$/;
    return pattern.test(value.trim());
}