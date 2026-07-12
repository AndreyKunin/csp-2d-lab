import React, {useCallback, useEffect, useMemo, useState} from 'react';
import {DashboardState} from "../../services/status";
import {
    continueSimulation,
    deleteSimulation,
    fetchSimulationResults,
    pauseSimulation,
    SimulationResults
} from "../../services/requests";

interface PlayerPanelProps {
    dashboardState: DashboardState;
    onResultsHandler: (results: SimulationResults) => void;
}

interface PlayerButtonProps {
    defaultPressed: boolean;
    disabled: boolean;
    prefix: string;
    altText: string;
    onClickHandler: () => void;
}

export const PlayerButton: React.FC<PlayerButtonProps> = ({ defaultPressed, disabled, prefix, altText, onClickHandler }) => {
    const [isPressed, setIsPressed] = useState(defaultPressed);

    const clickHandler = useCallback(() => {
        if (!disabled) {
            onClickHandler();
        }
    }, [defaultPressed, disabled, onClickHandler]);

    useEffect(() => {
        setIsPressed(defaultPressed);
    }, [defaultPressed]);

    return (
        <img
            className={disabled ? 'playerButtonDisabled' : 'playerButton'}
            src={`/player/${prefix}${isPressed ? '-inset' : ''}.svg`}
            onMouseDown={(e) => {
                if (e.button === 0 && !disabled) setIsPressed(true);
            }}
            onMouseUp={(e) => {
                if (e.button === 0 && !disabled) setIsPressed(defaultPressed);
            }}
            onMouseLeave={() => {
                if (!disabled) setIsPressed(defaultPressed)
            }}
            onClick={clickHandler}
            alt={altText}
        />
    )
}

export const PlayerPanel: React.FC<PlayerPanelProps> = ({ dashboardState, onResultsHandler }) => {
    const [status, setStatus] = useState(dashboardState.getStatus());
    const inProgress = useMemo(() => status === 'running', [status]);
    const isFinished = useMemo(() => status === 'completed' || status === 'error', [status]);

    useEffect(() => {
        const unsubscribe = dashboardState.onStatusChange(() => {
            setStatus(dashboardState.getStatus());
            if (dashboardState.getStatus() === 'completed') {
                finishJob();
            }
        });
        return () => unsubscribe();
    }, [dashboardState]);

    const onStart = useCallback(async () => {
        if (dashboardState.getStatus() === "paused") {
            await continueSimulation(dashboardState.getSimulationId());
        }
        await dashboardState.refresh();
    }, [dashboardState]);

    const onPause = useCallback(async () => {
        if (dashboardState.getStatus() !== "paused") {
            await pauseSimulation(dashboardState.getSimulationId());
        }
        await dashboardState.refresh();
    }, [dashboardState]);

    const onStop = useCallback(async () => {
        if (dashboardState.getStatus() !== "paused") {
            await pauseSimulation(dashboardState.getSimulationId());
        }
        await dashboardState.finishMonitoring();
        await finishJob();
    }, [dashboardState, onResultsHandler]);

    const finishJob = useCallback(async () => {
        const results = await fetchSimulationResults(dashboardState.getSimulationId());
        await deleteSimulation(dashboardState.getSimulationId());
        onResultsHandler(results);
    }, [dashboardState, onResultsHandler]);

    return (
        <div className="dashboardPlayerPanel">
            <PlayerButton prefix="play" altText="Play" defaultPressed={inProgress && !isFinished} onClickHandler={onStart} disabled={inProgress || isFinished} />
            <PlayerButton prefix="pause" altText="Pause" defaultPressed={!inProgress && !isFinished} onClickHandler={onPause} disabled={!inProgress || isFinished} />
            <PlayerButton prefix="stop" altText="Stop" defaultPressed={false} onClickHandler={onStop} disabled={isFinished} />
        </div>
    );
};