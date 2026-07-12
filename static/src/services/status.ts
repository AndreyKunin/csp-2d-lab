import {
    fetchSimulationStatus,
    SimulationJobFullStatus
} from "./requests";
import {
    ProblemInsights,
    SimulationRuntimeParameters,
    SimulationStatistics,
    SimulationStatus
} from "../utils/simulation";

export class DashboardState {
    private readonly simulationId: string;
    private readonly initialStatus: SimulationStatus;
    private labMode: boolean = false;
    private dataPoints: number = 20;
    private jobStatus?: SimulationJobFullStatus;
    private intervalId: number | null = null;
    private refreshPromise: Promise<void> | null = null;
    private readonly onError: (error: unknown) => void;

    private controlSubscribers: Set<() => void> = new Set<() => void>();
    private statusSubscribers: Set<() => void> = new Set<() => void>();


    constructor(simulationId: string, initialStatus: SimulationStatus, labMode: boolean, onError: (error: unknown) => void) {
        this.simulationId = simulationId;
        this.initialStatus = initialStatus;
        this.labMode = labMode;
        this.onError = onError;
        this.startPolling();
    }

    public getStatistics(): SimulationStatistics | undefined {
        return this.jobStatus?.statistics;
    }

    public getProblem(): ProblemInsights | undefined {
        return this.jobStatus?.problem;
    }

    public getJobStatus(): SimulationJobFullStatus | undefined {
        return this.jobStatus;
    }

    public getStatus(): SimulationStatus | undefined {
        return this.jobStatus ? this.jobStatus.status : this.initialStatus;
    }

    public getRuntimeParameters(): SimulationRuntimeParameters | undefined {
        return this.jobStatus?.statistics?.parameters;
    }

    public getSimulationId(): string {
        return this.simulationId;
    }

    public isLabMode(): boolean {
        return this.labMode;
    }

    public getDataPoints(): number {
        return this.dataPoints;
    }

    public startPolling() {
        if (!this.intervalId) {
            this.intervalId = window.setInterval(async () => {
                await this.fetchStatus(this.simulationId);
            }, 1000);
        }
    }

    public async refresh() {
        await this.fetchStatus(this.simulationId);
    }

    public async finishMonitoring() {
        this.stopPolling();
        if (this.jobStatus && (this.jobStatus.status === "running" || this.jobStatus.status === "paused")) {
            this.jobStatus.status = "completed";
            this.notifyStatusSubscribers();
        }
    }


    public stopPolling() {
        if (this.intervalId) {
            window.clearInterval(this.intervalId);
            this.intervalId = null;
        }
    }

    public onControlStateChange(callback: () => void) {
        this.controlSubscribers.add(callback);
        return () => {
            this.controlSubscribers.delete(callback);
        }
    }

    public onStatusChange(callback: () => void) {
        this.statusSubscribers.add(callback);
        return () => {
            this.statusSubscribers.delete(callback);
        }
    }

    private notifyControlSubscribers() {
        this.controlSubscribers.forEach(callback => callback());
    }

    private notifyStatusSubscribers() {
        this.statusSubscribers.forEach(callback => callback());
    }

    public async setLabMode(labMode: boolean) {
        if (this.labMode === labMode) {
            return;
        }
        this.labMode = labMode;
        this.notifyControlSubscribers();
    };

    public setDataPoints(dataPoints: number) {
        if (this.dataPoints === dataPoints) {
            return;
        }
        this.dataPoints = dataPoints;
        this.notifyControlSubscribers();
    };

    private async fetchStatus(id: string){
        if (this.refreshPromise) {
            return this.refreshPromise;
        }
        this.refreshPromise = (async () => {
            try {
                const response = await fetchSimulationStatus(id);
                this.jobStatus = response;
                this.notifyStatusSubscribers();
                if (response.error) {
                    this.onError(response.error);
                }
                if (response.status === "completed" || response.status === "error") {
                    await this.finishMonitoring();
                }
            } catch (error) {
                this.onError(error);
            } finally {
                this.refreshPromise = null;
            }
        })();

        return this.refreshPromise;
    }
}