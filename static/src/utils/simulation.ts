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
        this.checkCrossoverRate(errors, this.initialParameters.crossoverRate);

        this.checkRange(
            'Window size', 'Number of generations',
            errors, 1, Number.MAX_SAFE_INTEGER,
            this.initialParameters.windowSize, this.initialParameters.maxEpochsCount
        );

        this.checkCycleCount(errors, this.initialParameters.cycleCount);
        this.checkWholeAtLeast('Tournament size', errors, 1, this.initialParameters.tournamentSize);
        this.checkWholeAtLeast('Repair weight', errors, 1, this.initialParameters.repairWeight);
        this.checkWholeAtLeast('Flip weight', errors, 1, this.initialParameters.flipWeight);
        this.checkDonorFraction(errors, this.initialParameters.donorFraction);

        return errors;
    }

    // Cycles before the championship: a whole number, 0 or more (0 = the run
    // ends at the first stagnation).
    private checkCycleCount(errors: string[], value?: number) {
        if (value === undefined || isNaN(value) || !Number.isInteger(value)) {
            errors.push('Invalid Cycles before championship value.');
        } else if (value < 0) {
            errors.push('Cycles before championship cannot be less than 0.');
        }
    }

    private checkWholeAtLeast(name: string, errors: string[], min: number, value?: number) {
        if (value === undefined || isNaN(value) || !Number.isInteger(value)) {
            errors.push(`Invalid ${name} value.`);
        } else if (value < min) {
            errors.push(`${name} cannot be less than ${min}.`);
        }
    }

    // The donor fraction may be 1 (a whole parent can be copied).
    private checkDonorFraction(errors: string[], value?: number) {
        if (value === undefined || isNaN(value)) {
            errors.push('Invalid Donor fraction value.');
        } else if (value <= 0) {
            errors.push('Donor fraction must be greater than 0.');
        } else if (value > 1) {
            errors.push('Donor fraction cannot be greater than 1.');
        }
    }

    // The crossover rate may be 1 (every non-elite slot is filled by crossover).
    private checkCrossoverRate(errors: string[], value?: number) {
        if (value === undefined || isNaN(value)) {
            errors.push('Invalid Crossover rate value.');
        } else if (value <= 0) {
            errors.push('Crossover rate must be greater than 0.');
        } else if (value > 1) {
            errors.push('Crossover rate cannot be greater than 1.');
        }
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