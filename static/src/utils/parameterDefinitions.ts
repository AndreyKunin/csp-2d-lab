// Simulation parameters shown in the start form ("custom simulation parameters",
// all of them) and in the Lab Edit dialog (those with runtime: true).
//
// The default values and the validation rules come from the server
// (application.yaml; the server validates requests with the same rules):
// GET /api/v1/service/parameter-definitions, loaded by InputForm. The rules are
// set by setParameterRules, the defaults built by defaultParameterValues.
//
// Rule DSL: <type><open><min>-<max><close>
//   type   int (whole number) or float
//   open   [ = min inclusive, ( = min exclusive
//   close  ] = max inclusive, ) = max exclusive
//   min, max  non-negative numbers; an empty bound means unbounded
// Examples: float[0-1] = 0 <= v <= 1; int(0-100) = 0 < v < 100, whole;
//           int[2-) = whole, at least 2.

export interface ParameterDefinition {
    key: string;          // request field; a dotted key is a nested field (populationSize.min)
    displayName: string;
    runtime: boolean;     // also editable in the Lab Edit dialog
    tooltip?: string;
}

const populationSizeTooltip = 'Population size a number of solutions within the population, which is maintained across generations. This value is adaptive and depends on the problem size. However, we set boundaries for minimum and maximum values to reduce compute requirements.';

export const PARAMETER_DEFINITIONS: ParameterDefinition[] = [
    {
        key: 'populationSize.min', displayName: 'Minimum population size', runtime: false,
        tooltip: populationSizeTooltip,
    },
    {
        key: 'populationSize.max', displayName: 'Maximum population size', runtime: false,
        tooltip: populationSizeTooltip,
    },
    {
        key: 'eliteCount', displayName: 'Elite count', runtime: true,
        tooltip: 'The elite size specifies the number of the fittest individuals that are guaranteed to be carried over from one generation to the next without any changes.',
    },
    {
        key: 'mutationRate', displayName: 'Mutation rate', runtime: true,
        tooltip: 'Mutation rate is the probability that a single element of the solution will be altered during the creation of a new offspring solution.',
    },
    {
        key: 'crossoverRate', displayName: 'Crossover rate', runtime: true,
        tooltip: 'Share of the non-elite offspring made by crossover, greater than 0 and at most 1. The rest are copies of tournament-selected individuals, changed by mutation only.',
    },
    {
        key: 'maxEpochsCount', displayName: 'Number of generations', runtime: true,
        tooltip: 'Maximum nimber of epochs (generations) in the simulation. Simulation is a process that never halts until manually stopped. We can auto-stop the process when it reaches the maximum number of epochs.',
    },
    {
        key: 'windowSize', displayName: 'Window size', runtime: false,
        tooltip: 'Number of epochs without improvement of the best solution that ends a cycle (and the championship). Also the length of the sliding window that smooths the charted metrics. From 1 to 250.',
    },
    {
        key: 'cycleCount', displayName: 'Cycles before championship', runtime: true,
        tooltip: 'When the best solution stops improving for a window, the cycle ends: its elite is kept and the simulation starts over from random solutions. After this many cycles, all kept elites compete in a final championship; the simulation ends when it stops improving too.',
    },
    {
        key: 'tournamentSize', displayName: 'Tournament size', runtime: true,
        tooltip: 'Number of individuals drawn at random for each parent; the best of them becomes the parent. Larger values favour the best solutions more strongly. A whole number, 2 or more.',
    },
    {
        key: 'repairWeight', displayName: 'Repair weight', runtime: true,
        tooltip: 'Mutation budget units that one successful eliminate-and-repair mutation costs (the worst sheet is emptied and its parts are packed again). Larger values mean fewer mutations per generation. A whole number, 1 or more.',
    },
    {
        key: 'flipWeight', displayName: 'Flip weight', runtime: true,
        tooltip: 'Mutation budget units that one successful operator flip costs (a cut direction that wastes the most area is flipped). Larger values mean fewer mutations per generation. A whole number, 1 or more.',
    },
    {
        key: 'donorFraction', displayName: 'Donor fraction', runtime: true,
        tooltip: 'Largest part of a parent that crossover copies into a child, as a share of the parts: greater than 0 and at most 1.',
    },
];

// overriddenParameters returns the keys the server replaces for n parts, so
// the start form disables them and does not send them. It mirrors
// smallInputOverrides in mvc/request.go: for n in [4, 10] the population
// minimum is n²/2 and the elite count is 1 (n = 4) or 2. Below 4 parts the
// input is invalid (Model.validate), so nothing is overridden.
export function overriddenParameters(n: number): Set<string> {
    const keys = new Set<string>();
    if (n < 4) {
        return keys;
    }
    if (n <= 10) {
        keys.add('populationSize.min');
        keys.add('eliteCount');
    }
    return keys;
}

// ---- Rule DSL ----------------------------------------------------------------

export interface ParsedRule {
    type: 'int' | 'float';
    min: number;          // -Infinity when unbounded
    minInclusive: boolean;
    max: number;          // +Infinity when unbounded
    maxInclusive: boolean;
}

const RULE_PATTERN = /^(int|float)([[(])(\d+(?:\.\d+)?)?-(\d+(?:\.\d+)?)?([\])])$/;

export function parseRule(rule: string): ParsedRule {
    const m = RULE_PATTERN.exec(rule.trim());
    if (!m) {
        throw new Error(`Invalid parameter rule "${rule}"`);
    }
    const parsed: ParsedRule = {
        type: m[1] as 'int' | 'float',
        min: m[3] === undefined ? -Infinity : Number(m[3]),
        minInclusive: m[2] === '[',
        max: m[4] === undefined ? Infinity : Number(m[4]),
        maxInclusive: m[5] === ']',
    };
    if (parsed.min > parsed.max) {
        throw new Error(`Invalid parameter rule "${rule}": min is greater than max`);
    }
    return parsed;
}

const INT_PATTERN = /^-?\d+$/;
const FLOAT_PATTERN = /^-?(\d+(\.\d*)?|\.\d+)$/;

// describeRule renders a rule for error messages, e.g. "a whole number from 1
// to 250", "a number from 0 to 1", "a whole number, at least 2".
export function describeRule(rule: ParsedRule): string {
    const kind = rule.type === 'int' ? 'a whole number' : 'a number';
    const hasMin = rule.min !== -Infinity, hasMax = rule.max !== Infinity;
    if (hasMin && hasMax && rule.minInclusive && rule.maxInclusive) {
        return `${kind} from ${rule.min} to ${rule.max}`;
    }
    const parts: string[] = [];
    if (hasMin) {
        parts.push(rule.minInclusive ? `at least ${rule.min}` : `greater than ${rule.min}`);
    }
    if (hasMax) {
        parts.push(rule.maxInclusive ? `at most ${rule.max}` : `less than ${rule.max}`);
    }
    return parts.length ? `${kind}, ${parts.join(' and ')}` : kind;
}

// checkRuleValue returns null when value (as typed) satisfies the rule, else
// the reason ("must be ...").
export function checkRuleValue(rule: ParsedRule, value: string): string | null {
    const text = value.trim();
    const pattern = rule.type === 'int' ? INT_PATTERN : FLOAT_PATTERN;
    const reason = `must be ${describeRule(rule)}`;
    if (!pattern.test(text)) {
        return reason;
    }
    const v = Number(text);
    if (isNaN(v)) {
        return reason;
    }
    if (rule.min !== -Infinity && (rule.minInclusive ? v < rule.min : v <= rule.min)) {
        return reason;
    }
    if (rule.max !== Infinity && (rule.maxInclusive ? v > rule.max : v >= rule.max)) {
        return reason;
    }
    return null;
}

const DEFINITIONS = new Map<string, ParameterDefinition>(PARAMETER_DEFINITIONS.map(d => [d.key, d]));

// What the server sends of each parameter (GET parameter-definitions), by key.
export interface ServerParameterDefinition {
    default: number;
    rule: string;         // DSL above
}
export type ServerParameterDefinitions = Record<string, ServerParameterDefinition>;

// The server's rules, parsed once when they are loaded (setParameterRules).
const RULES = new Map<string, ParsedRule>();

// setParameterRules parses the rules the server sent. A malformed rule or a
// definition without a rule is a programming error: it is logged, and
// validateParameter then reports the parameter as not validatable.
export function setParameterRules(definitions: ServerParameterDefinitions) {
    RULES.clear();
    for (const [key, definition] of Object.entries(definitions)) {
        try {
            RULES.set(key, parseRule(definition.rule));
        } catch (error) {
            console.error(error);
        }
    }
    for (const definition of PARAMETER_DEFINITIONS) {
        if (!RULES.has(definition.key)) {
            console.error(`No validation rule for parameter ${definition.key}`);
        }
    }
}

export function findParameter(key: string): { definition: ParameterDefinition, rule: ParsedRule } | undefined {
    const definition = DEFINITIONS.get(key);
    const rule = RULES.get(key);
    return definition && rule ? {definition, rule} : undefined;
}

// validateParameter returns null when the value of the parameter `key` is
// valid, else an error message for the user.
export function validateParameter(key: string, value: string): string | null {
    const found = findParameter(key);
    if (!found) {
        return DEFINITIONS.has(key) ? `No validation rule for ${key} from the server.` : `Unknown parameter ${key}.`;
    }
    const reason = checkRuleValue(found.rule, value);
    return reason ? `${found.definition.displayName} ${reason}.` : null;
}

// valueOf reads the field `key` (dotted for nested fields) of a parameter
// object; undefined when absent.
export function valueOf(source: object | undefined, key: string): unknown {
    let current: unknown = source;
    for (const part of key.split('.')) {
        if (current === null || typeof current !== 'object') {
            return undefined;
        }
        current = (current as Record<string, unknown>)[part];
    }
    return current;
}

// defaultParameterValues builds the parameter object (dotted keys become nested
// fields: populationSize.min) from the server's defaults.
export function defaultParameterValues(definitions: ServerParameterDefinitions): Record<string, unknown> {
    const parameters: Record<string, unknown> = {};
    for (const [key, definition] of Object.entries(definitions)) {
        const path = key.split('.');
        let target = parameters;
        for (const part of path.slice(0, -1)) {
            if (typeof target[part] !== 'object' || target[part] === null) {
                target[part] = {};
            }
            target = target[part] as Record<string, unknown>;
        }
        target[path[path.length - 1]] = definition.default;
    }
    return parameters;
}
