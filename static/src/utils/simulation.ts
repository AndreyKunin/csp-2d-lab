import {InputFormData} from "../components/InputForm";
import {isNaN} from "mathjs";
import {getSimulationParameters} from "./parameters";

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
    saturatedSince: number;
    convergedSince: number;
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
    maxRate?: number;
    minRate?: number;
    maxEpochsCount?: number;
    windowSize?: number;
}

export interface SimulationRuntimeParameters {
    // Run-time parameters snapshot from the Statistics output. All fields are mandatory.
    populationSize: number;
    eliteCount: number;
    mutationRate: number;
    maxRate: number;
    minRate: number;
    maxEpochsCount: number;
    windowSize: number;
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

    public validate(): string[] {
        let errors: string[] = [];

        if (!this.initialParameters) {
            return errors;
        }

        this.checkRange(
            'Minimum population size', 'Maximum population size',
            errors, 3, Number.MAX_SAFE_INTEGER,
            this.initialParameters.populationSize?.min, this.initialParameters.populationSize?.max
        );

        this.checkRange(
            'Elite count', 'Minimum population size',
            errors, 1, Number.MAX_SAFE_INTEGER,
            this.initialParameters.eliteCount, this.initialParameters.populationSize?.min
        );

        this.checkRate("Mutation rate", errors, this.initialParameters.mutationRate);
        this.checkRate("Minimum rate", errors, this.initialParameters.minRate);
        this.checkRate("Maximum rate", errors, this.initialParameters.maxRate);

        this.checkRange(
            'Window size', 'Number of generations',
            errors, 1, Number.MAX_SAFE_INTEGER,
            this.initialParameters.windowSize, this.initialParameters.maxEpochsCount
        );

        return errors;
    }

    private checkRate(name: string, errors: string[], value?: number) {
        if (value === undefined || isNaN(value)) {
            errors.push(`Invalid ${name} value.`);
        } else if (value === 0) {
            errors.push(`${name} cannot be 0.`);
        } else if (value >= 1) {
            errors.push(`${name} must be less than 1.`);
        }
    }

    private checkRange(nameLow: string, nameHigh: string, errors: string[], min: number, max: number, valueLow?: number, valueHigh?: number) {
        let formatValid = true;
        if (valueLow === undefined || isNaN(valueLow)) {
            errors.push(`Invalid ${nameLow} value.`);
            formatValid = false;
        }
        if (valueHigh === undefined || isNaN(valueHigh)) {
            errors.push(`Invalid ${nameHigh} value.`);
            formatValid = false;
        }
        if (!formatValid) {
            return;
        }

        if (valueLow !== undefined && valueLow < min) {
            errors.push(`${nameLow} cannot be less than ${min}.`);
        }
        if (valueHigh !== undefined && valueHigh > max) {
            errors.push(`${nameHigh} cannot be higher than ${max}.`);
        }
        if (valueLow !== undefined && valueHigh !== undefined && valueLow > valueHigh) {
            errors.push(`${nameLow} is greater than ${nameHigh}.`);
        }
    }
}