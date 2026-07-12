import React, {useState, useEffect, useMemo, useRef, useCallback} from 'react';
import {PropertyGrid} from "../PropertyGrid";
import {SimulationParameters, SimulationRuntimeParameters} from "../../utils/simulation";
import {
    getRuntimeParameters, getSimulationParameters,
    PropertyItem,
    validateSimulationParameters
} from "../../utils/parameters";
import {DashboardState} from "../../services/status";

interface EditDialogProps {
    dashboardState: DashboardState;
    isOpen: boolean;
    onClose: () => void;
    onSubmit: (parameters: SimulationParameters) => void;
}

export const EditDialog: React.FC<EditDialogProps> = ({ dashboardState, isOpen, onClose, onSubmit }) => {
    const [animationStep, setAnimationStep] = useState(0);
    const [simulationParameters, setSimulationParameters] = useState<PropertyItem[]>(getRuntimeParameters(dashboardState.getRuntimeParameters()));
    const simulationParametersValidationErrors = useRef<Map<string, boolean>>(new Map());

    useEffect(() => {
        if (isOpen) {
            // Start animation sequence
            setAnimationStep(1); // dot
            setSimulationParameters(getRuntimeParameters(dashboardState.getRuntimeParameters()));
            setTimeout(() => setAnimationStep(2), 100); // expand horizontally to line
            setTimeout(() => setAnimationStep(3), 300); // expand vertically to rectangle
        } else {
            setAnimationStep(0);
        }
    }, [isOpen, dashboardState]);

    const handleClear = useCallback(() => {
        setSimulationParameters(getRuntimeParameters(dashboardState.getRuntimeParameters()));
    }, [dashboardState]);

    const handleCancel = useCallback(() => {
        handleClear();
        onClose();
    }, [handleClear, onClose]);

    const handleSubmit = (e: React.FormEvent) => {
        e.preventDefault();
        const parameterResults = simulationParameters.map((row, i) => {
            return validateSimulationParameters(false, row.key, row.value, simulationParametersValidationErrors);
        });
        const parameterDataValid = parameterResults && parameterResults.every(r => r);
        if (parameterDataValid) {
            onSubmit(getSimulationParameters(simulationParameters));
        }
    };

    const updateSimulationParameters = useCallback((rowIndex: number, value: string) => {
        const newRows = [...simulationParameters];
        const key = newRows[rowIndex].key;
        newRows[rowIndex].value = value;
        setSimulationParameters(newRows);
        validateSimulationParameters(true, key, value, simulationParametersValidationErrors);
    }, [simulationParameters]);

    if (!isOpen) {
        return null;
    }

    return (
        <div className="dialog-overlay">
            <div className={`dialog-container step-${animationStep}`}>
                <div className="dialog-content">
                    <form className="dialog-form">
                        <div>
                            <span className="sectionHeader">Simulation parameters</span>
                        </div>
                        <PropertyGrid
                            id="r_parameters"
                            data={simulationParameters}
                            validationErrors={simulationParametersValidationErrors.current}
                            update={updateSimulationParameters}
                        />
                        <div className="sectionSpacer">&nbsp;</div>
                        <div className="actions">
                            <div onClick={handleClear}>Clear</div>
                            <div onClick={handleCancel}>Cancel</div>
                            <div onClick={handleSubmit}>Submit</div>
                        </div>
                    </form>
                </div>
            </div>
        </div>
    );
};