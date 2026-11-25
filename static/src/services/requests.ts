import {Model, Solution} from '../utils/model';
import {
    ProblemInsights,
    Simulation,
    SimulationParameters,
    SimulationStatistics,
    SimulationStatus
} from "../utils/simulation";
import {canvasesToPngBlobs} from "../utils/layout";
import {ServerParameterDefinitions} from "../utils/parameterDefinitions";

interface StartRequest {
    sheets?: Sheet;
    parts: Part[];
    sawWidth?: number;
    parameters?: SimulationParameters;
}

// The API (mvc.ConfigureRoutes): PUT starts a simulation; /<id> takes GET
// (?format=status|result), POST (an action) and DELETE.
const SIMULATION_PATH = '/api/v1/simulation';

// The body of POST /api/v1/simulation/<id>; parameters for "configure" only.
interface ActionRequest {
    action: 'pause' | 'continue' | 'configure';
    parameters?: SimulationParameters;
}

const postAction = async (id: string, request: ActionRequest) => {
    const response = await fetch(`${SIMULATION_PATH}/${id}`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify(request),
    });
    return validateAndParse(response);
};

interface Part {
    label: string;
    width: number;
    length: number;
    count: number;
}

interface Sheet {
    width: number;
    length: number;
}

export class SimulationJobStatus {
    id: string;
    status: SimulationStatus;
    created_at: string;
    updated_at: string;
    message: string;
    error?: string;

    constructor(data: any) {
        this.id = data.id;
        this.status = data.status;
        this.created_at = data.created_at;
        this.updated_at = data.updated_at;
        this.message = data.message;
        this.error = data.error;
    }

    public enrich(simulation: Simulation) {
        simulation.id = this.id;
        simulation.status = this.status;
        simulation.lastMessage = this.message;
        simulation.error = this.error;
    }
}

export class SimulationJobFullStatus extends SimulationJobStatus {
    statistics?: SimulationStatistics;
    problem?: ProblemInsights;
    timestamp: Date;

    constructor(data: any) {
        super(data);
        this.statistics = data.statistics;
        this.problem = data.problem;
        this.timestamp = new Date();
    }
}

export class SimulationResults {
    solutions: Solution[];

    constructor(data: any) {
        this.solutions = data.solutions;
    }
}


// getParameterDefinitions returns the server's default value and validation
// rule of every parameter (key -> {default, rule}; the rule in the DSL of
// utils/parameterDefinitions.ts).
export const getParameterDefinitions = async () => {
    try {
        const response = await fetch('/api/service/v1/parameter-definitions', {
            method: 'GET',
            headers: {
                'Content-Type': 'application/json',
            },
        });

        await validate(response);

        const definitions: ServerParameterDefinitions = await response.json();

        return definitions;
    } catch (error) {
        console.error('API request failed:', error);
        throw error;
    }
}

export const configureSimulation = async (id: string, parameters: SimulationParameters) => {
    try {
        return await postAction(id, {action: 'configure', parameters});
    } catch (error) {
        console.error('Configure request failed:', error);
        throw error;
    }
};

export const startSimulation = async (model: Model, parameters?: SimulationParameters) => {
    try {
        const numericSheets = model.getNumericSheets(true);
        const numericParts = model.getNumericParts(true);
        const request: StartRequest = {
            parts: []
        }
        if (numericSheets) {
            request.sheets = {
                width: numericSheets.width as number,
                length: numericSheets.length as number
            }
        }
        for (let i = 0; i < numericParts.length; ++i) {
            request.parts.push({
                label: numericParts[i].label,
                width: numericParts[i].width as number,
                length: numericParts[i].length as number,
                count: numericParts[i].count as number,
            })
        }

        request.sawWidth = model.getNumber(model.sawWidth, true) as number;
        request.parameters = parameters;

        const response = await fetch(SIMULATION_PATH, {
            method: 'PUT',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(request),
        });

        return validateAndParse(response);
    } catch (error) {
        console.error('Start request failed:', error);
        throw error;
    }
};

export const pauseSimulation = async (id: string) => {
    try {
        return await postAction(id, {action: 'pause'});
    } catch (error) {
        console.error('Pause request failed:', error);
        throw error;
    }
};

export const continueSimulation = async (id: string) => {
    try {
        return await postAction(id, {action: 'continue'});
    } catch (error) {
        console.error('Continue request failed:', error);
        throw error;
    }
};

export const deleteSimulation = async (id: string) => {
    try {
        console.log("Deleting simulation with id: " + id);
        const response = await fetch(`${SIMULATION_PATH}/${id}`, {
            method: 'DELETE',
            headers: {
                'Content-Type': 'application/json',
            },
        });
        return validateAndParse(response);
    } catch (error) {
        console.error('Stop request failed:', error);
        throw error;
    }
};


export const fetchSimulationStatus = async (id: string) => {
    try {
        const response = await fetch(`${SIMULATION_PATH}/${id}?format=status`, {
            method: 'GET',
            headers: {
                'Content-Type': 'application/json',
            },
        });

        await validate(response);
        const data: any = await response.json();

        return new SimulationJobFullStatus(data);

    } catch (error) {
        console.error('Status request failed:', error);
        throw error;
    }
}


export const fetchSimulationResults = async (id: string) => {
    try {
        const response = await fetch(`${SIMULATION_PATH}/${id}?format=result`, {
            method: 'GET',
            headers: {
                'Content-Type': 'application/json',
            },
        });

        await validate(response);
        const data: any = await response.json();

        return new SimulationResults(data);

    } catch (error) {
        console.error('History request failed:', error);
        throw error;
    }
}

export const generatePdf = async (model: Model, canvases: HTMLCanvasElement[]): Promise<Blob> => {
    const numericSheets = model.getNumericSheets(true);
    const numericParts = model.getNumericParts(true);

    const request: StartRequest = { parts: [] };
    if (numericSheets) {
        request.sheets = {
            width: numericSheets.width as number,
            length: numericSheets.length as number,
        };
    }
    for (const part of numericParts) {
        request.parts.push({
            label: part.label,
            width: part.width as number,
            length: part.length as number,
            count: part.count as number,
        });
    }
    request.sawWidth = model.getNumber(model.sawWidth, true) as number;

    const blobs = await canvasesToPngBlobs(canvases);

    const formData = new FormData();
    formData.append('request', JSON.stringify(request));
    blobs.forEach((blob, idx) => {
        const filename = `sheet-${idx + 1}.png`;
        formData.append('images', new File([blob], filename, { type: 'image/png' }), filename);
    });

    const response = await fetch('/api/v1/report', {
        method: 'POST',
        body: formData,
        // Do NOT set Content-Type manually; the browser sets the multipart boundary.
    });

    if (!response.ok) {
        let message = `Server error! Status: ${response.status}.`;
        try {
            const txt = await response.text();
            if (txt) message += ' ' + txt;
        } catch { /* ignore */ }
        throw new Error(message);
    }

    return await response.blob();
};


async function validateAndParse(response: Response) {
    await validate(response);
    const data: any = await response.json();

    const simulationJobStatus = new SimulationJobStatus(data);
    console.log(simulationJobStatus.message);

    return simulationJobStatus;
}

async function validate(response: Response) {
    if (!response.ok) {
        let errorMessage = `Server error! Status: ${response.status}. `;
        try {
            const errorData = await response.json();
            if (errorData.message) {
                errorMessage += errorData.message;
            }
        } catch (parseError) {
            errorMessage += response.statusText;
        }
        throw new Error(errorMessage);
    }
}