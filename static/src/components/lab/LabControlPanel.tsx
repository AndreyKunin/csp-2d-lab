import React, {useCallback, useEffect, useMemo, useState} from 'react';
import {ToggleSwitch} from "./ToggleSwitch";
import {ThreePhaseKnob} from "./ThreePhaseKnob";
import {PlayerButton} from "./PlayerPanel";
import {EditDialog} from "./EditDialog";
import {SimulationParameters} from "../../utils/simulation";
import {DashboardState} from "../../services/status";
import {configureSimulation} from "../../services/requests";

interface ControlPanelProps {
    dashboardState: DashboardState;
    onLabClickHandler: (isLab: boolean) => void;
}



export const LabControlPanel: React.FC<ControlPanelProps> = (
    {dashboardState, onLabClickHandler }
) => {

    const [status, setStatus] = useState(dashboardState.getStatus());
    const [dataPoints, setDataPoints] = useState(dashboardState.getDataPoints());

    useEffect(() => {
        const unsubscribeStatus = dashboardState.onStatusChange(() => {
            setStatus(dashboardState.getStatus());
        });
        const unsubscribeControlState = dashboardState.onControlStateChange(() => {
            setDataPoints(dashboardState.getDataPoints());
        });
        return () => {
            unsubscribeStatus();
            unsubscribeControlState();
        }
    }, [dashboardState]);

    const canEditParameters = useMemo(() => status === "paused"
        , [status]);

    const [editDialogOpen, setEditDialogOpen] = useState<boolean>(false);

    const onEdit = useCallback(() => {
        setEditDialogOpen(true);
    }, []);

    const onEditDialogSubmit = useCallback(async (parameters: SimulationParameters) => {
        await configureSimulation(dashboardState.getSimulationId(), parameters);
        await dashboardState.refresh();
        setEditDialogOpen(false);
    }, []);

    const onEditDialogCancel = useCallback(() => {
        setEditDialogOpen(false);
    }, []);

    return (
        <div className="dashboardControlPanel labControlPanel">
            <div className="controlPanelSplitColumns">
                <ToggleSwitch
                    altText="Switch to monitoring mode"
                    isOn={true}
                    className={"switchWrapperSingle"}
                    onText="Lab mode"
                    offText="Monitoring mode"
                    onClickHandler={onLabClickHandler}
                />
            </div>
            <div className="controlPanelSplitControls">
                <ThreePhaseKnob
                    values={[20, 50, 100]}
                    index={dataPoints === 20 ? 0 : dataPoints === 50 ? 1 : 2}
                    clickHandler={(m: number) => dashboardState.setDataPoints(m)}
                    title="Data points"
                    altText="Switch to display N latest events"
                />
                <div className="controlPanelButton">
                    <div>
                        <PlayerButton
                            prefix="edit"
                            altText="Edit"
                            defaultPressed={editDialogOpen}
                            onClickHandler={onEdit}
                            disabled={!canEditParameters}
                        />
                    </div>
                    <EditDialog
                        dashboardState={dashboardState}
                        isOpen={canEditParameters && editDialogOpen}
                        onClose={onEditDialogCancel}
                        onSubmit={onEditDialogSubmit}
                    />
                </div>
            </div>
        </div>
    );
};