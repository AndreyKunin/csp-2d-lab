import React, {useCallback, useMemo, useState} from 'react';
import {SimulationStatistics, SimulationStatus} from "../../utils/simulation";
import {ToggleSwitch} from "./ToggleSwitch";

interface ControlPanelProps {
    onLabClickHandler: (isLab: boolean) => void;
}

export const MonitorControlPanel: React.FC<ControlPanelProps> = ({ onLabClickHandler }) => {

    return (
        <div className="dashboardControlPanel">
            <div className="controlPanelSplitColumns">
                <ToggleSwitch
                    altText="Switch to lab mode (advanced)"
                    isOn={false}
                    className={"switchWrapperSingle"}
                    onText="Lab mode (advanced)"
                    offText="Monitoring mode"
                    onClickHandler={onLabClickHandler}
                />
            </div>
        </div>
    );
};