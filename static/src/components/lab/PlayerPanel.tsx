import React, {useCallback, useEffect, useMemo, useRef, useState} from 'react';
import {DashboardState, finishOnce} from "../../services/status";
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

    // finishJob fetches the result, deletes the job and hands the result over. Stop
    // triggers it twice, from the status subscriber and from onStop, and both are
    // needed (before the first status poll the subscriber is not notified), so it
    // runs once (finishOnce, FE-04). It is created once per job: the subscriber
    // keeps the function of the first render, so a new one would not be shared;
    // the results handler is read through a ref.
    const onResultsRef = useRef(onResultsHandler);
    onResultsRef.current = onResultsHandler;
    const finishJob = useMemo(() => finishOnce(async () => {
        const results = await fetchSimulationResults(dashboardState.getSimulationId());
        await deleteSimulation(dashboardState.getSimulationId());
        onResultsRef.current(results);
    }), [dashboardState]);

    useEffect(() => {
        const unsubscribe = dashboardState.onStatusChange(() => {
            setStatus(dashboardState.getStatus());
            if (dashboardState.getStatus() === 'completed') {
                // onStop awaits the same run and sees its error; on a natural completion
                // nobody awaits it, so it is logged.
                finishJob().catch(error => console.error('Finishing the job failed:', error));
            }
        });
        return () => unsubscribe();
    }, [dashboardState, finishJob]);

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
        await finishJob(); // the same run as the subscriber's, if it started one
    }, [dashboardState, finishJob]);

    return (
        <div className="dashboardPlayerPanel">
            <PlayerButton prefix="play" altText="Play" defaultPressed={inProgress && !isFinished} onClickHandler={onStart} disabled={inProgress || isFinished} />
            <PlayerButton prefix="pause" altText="Pause" defaultPressed={!inProgress && !isFinished} onClickHandler={onPause} disabled={!inProgress || isFinished} />
            <PlayerButton prefix="stop" altText="Stop" defaultPressed={false} onClickHandler={onStop} disabled={isFinished} />
        </div>
    );
};