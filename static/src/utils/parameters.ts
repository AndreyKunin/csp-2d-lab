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

// validateSimulationParameters checks one parameter value against its rule
// (parameterDefinitions.ts) and records the result for the grid. allowEmpty
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