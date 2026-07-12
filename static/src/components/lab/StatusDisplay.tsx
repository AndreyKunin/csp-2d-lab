import React, {useEffect, useRef} from 'react';
import {SimulationStatus} from "../../utils/simulation";

interface StatusDisplayProps {
    status: SimulationStatus;
}

export const StatusDisplay: React.FC<StatusDisplayProps> = ({ status }) => {
    const greenLightRef = useRef<HTMLImageElement>(null);
    const redLightRef = useRef<HTMLImageElement>(null);

    useEffect(() => {
        const inProgress = status === 'running';
        const isError = status === 'error';
        const isPaused = status === 'paused';
        const isFinished = status === 'completed';
        let intervalId = null;
        if (inProgress && greenLightRef.current) {
            greenLightRef.current.setAttribute('src', '/lab/light-green-on.svg');
        } else if (isFinished && redLightRef.current) {
            redLightRef.current.setAttribute('src', '/lab/light-red-on.svg');
        } else if (isPaused || isError) {
            let greenOn = false, redOn = false;
            intervalId = window.setInterval(() => {
                if (isPaused && greenLightRef.current) {
                    greenLightRef.current.setAttribute('src', `/lab/light-green-${greenOn ? 'on' : 'off'}.svg`);
                    greenOn = !greenOn;
                } else if (isError && redLightRef.current) {
                    redLightRef.current.setAttribute('src', `/lab/light-red-${redOn ? 'on' : 'off'}.svg`);
                    redOn = !redOn;
                }
            }, 500);
            console.log('set interval');
        }

        return () => {
            if (intervalId) {
                window.clearInterval(intervalId);
                console.log('cleared interval');
            }
        }
    }, [status, greenLightRef, redLightRef]);

    return (
        <div className="led statusDisplay">
            <div>{status}</div>
            <div>
                <div className="indicator">
                    <img ref={greenLightRef} src="/lab/light-green-off.svg" alt="Running"/>
                </div>
                <div className="indicator">
                    <img ref={redLightRef} src="/lab/light-red-off.svg" alt="Idle"/>
                </div>
            </div>
        </div>
    );
};