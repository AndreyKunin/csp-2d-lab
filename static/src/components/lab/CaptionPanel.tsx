import React, {useCallback, useMemo, useState} from 'react';
import {SimulationStatistics, SimulationStatus} from "../../utils/simulation";
import {PlayerPanel} from "./PlayerPanel";

interface CaptionPanelProps {
    text: string;
}

export const CaptionPanel: React.FC<CaptionPanelProps> = ({ text }) => {

    return (
        <div className="dashboardCaptionPanel">
            {text}
        </div>
    );
};