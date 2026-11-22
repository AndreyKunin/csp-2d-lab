import {InputFormData} from "../components/InputForm";
import {getSimulationParameters} from "./parameters";
import {PARAMETER_DEFINITIONS, validateParameter, valueOf} from "./parameterDefinitions";

interface PopulationSize {
    min: number;
    max: number;
}

export type SimulationStatus = "running" | "paused" | "completed" | "error";

export interface ProblemInsights {
    bestFitnessVector: number[];
    selectedSpecies: number[][];
}

export interface EpochState {
    bestFitness: number;
    averageFitness: number;
    fitnessDeviation: number;
    averageEliteFitness: number;
    eliteFitnessDeviation: number;
    epochNumber: number;
    failures: number;
}

export interface WindowState {
    averageFitness: number;
    averageFitnessDeviation: number;
    averageEliteFitness: number;
    eliteFitnessDeviation: number;
    saturationRate: number;
}

export interface SimulationState {
    epochState: EpochState;
    windowState: WindowState;
    cycles: number;            // finished cycles (each ends when the best stagnates for windowSize epochs)
    championshipSince: number; // epoch the championship began; 0 = not yet
}

export interface SimulationStatistics {
    parameters?: SimulationRuntimeParameters;
    state?: SimulationState;
}

export interface SimulationParameters {
    // Parameters of the start request. All parameters are optional (The server has defaults).
    populationSize?: PopulationSize;
    eliteCount?: number;
    mutationRate?: number;
    crossoverRate?: number;
    maxEpochsCount?: number;
    windowSize?: number;
    cycleCount?: number;
    tournamentSize?: number;
    repairWeight?: number;
    flipWeight?: number;
    donorFraction?: number;
}

export interface SimulationRuntimeParameters {
    // Run-time parameters snapshot from the Statistics output. All fields are mandatory.
    populationSize: number;
    eliteCount: number;
    mutationRate: number;
    crossoverRate: number;
    maxEpochsCount: number;
    windowSize: number;
    cycleCount: number;
    tournamentSize: number;
    repairWeight: number;
    flipWeight: number;
    donorFraction: number;
}

export class Simulation {
    id?: string;
    initialParameters?: SimulationParameters;
    status?: SimulationStatus;
    lastMessage?: string;
    error?: string;
    statistics?: SimulationStatistics;
    problem?: ProblemInsights;
    startTime?: Date;

    constructor(formData: InputFormData) {
        if (formData.parameters) {
            this.initialParameters = getSimulationParameters(formData.parameters);
        }
    }

    // validate checks every parameter against its rule (parameterDefinitions.ts),
    // then the relations between parameters.
    public validate(): string[] {
        const errors: string[] = [];
        const p = this.initialParameters;
        if (!p) {
            return errors;
        }
        for (const definition of PARAMETER_DEFINITIONS) {
            const value = valueOf(p, definition.key);
            if (value === undefined) {
                continue; // not sent: overridden by the server (overriddenParameters)
            }
            const error = validateParameter(definition.key, value === null ? '' : String(value));
            if (error) {
                errors.push(error);
            }
        }
        if (errors.length > 0) {
            return errors;
        }
        const minPopulation = p.populationSize?.min, maxPopulation = p.populationSize?.max;
        if (minPopulation !== undefined && maxPopulation !== undefined && minPopulation > maxPopulation) {
            errors.push('Minimum population size is greater than Maximum population size.');
        }
        if (p.eliteCount !== undefined && minPopulation !== undefined && p.eliteCount > minPopulation) {
            errors.push('Elite count is greater than Minimum population size.');
        }
        if (p.windowSize !== undefined && p.maxEpochsCount !== undefined && p.windowSize > p.maxEpochsCount) {
            errors.push('Window size is greater than Number of generations.');
        }
        return errors;
    }
}