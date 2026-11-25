import React, {useState, useEffect, useMemo, useRef, useCallback} from 'react';
import {PropertyGrid} from "../PropertyGrid";
import {SimulationParameters, SimulationRuntimeParameters} from "../../utils/simulation";
import {
    getRuntimeParameters,
    PropertyItem,
    submitRuntimeParameters,
    validateSimulationParameters
} from "../../utils/parameters";
import {DashboardState} from "../../services/status";
import {ErrorList} from "../ErrorList";

interface EditDialogProps {
    dashboardState: DashboardState;
    isOpen: boolean;
    onClose: () => void;
    // onSubmit configures the job; a rejection (a failed configure) is shown in the dialog.
    onSubmit: (parameters: SimulationParameters) => Promise<void>;
}

export const EditDialog: React.FC<EditDialogProps> = ({ dashboardState, isOpen, onClose, onSubmit }) => {
    const [animationStep, setAnimationStep] = useState(0);
    const [simulationParameters, setSimulationParameters] = useState<PropertyItem[]>(getRuntimeParameters(dashboardState.getRuntimeParameters()));
    const simulationParametersValidationErrors = useRef<Map<string, boolean>>(new Map());
    const [errors, setErrors] = useState<string[]>([]); // why the values were rejected, or why the configure failed

    useEffect(() => {
        if (isOpen) {
            // Start animation sequence
            setAnimationStep(1); // dot
            setSimulationParameters(getRuntimeParameters(dashboardState.getRuntimeParameters()));
            setErrors([]);
            setTimeout(() => setAnimationStep(2), 100); // expand horizontally to line
            setTimeout(() => setAnimationStep(3), 300); // expand vertically to rectangle
        } else {
            setAnimationStep(0);
        }
    }, [isOpen, dashboardState]);

    const handleClear = useCallback(() => {
        setSimulationParameters(getRuntimeParameters(dashboardState.getRuntimeParameters()));
        setErrors([]);
    }, [dashboardState]);

    const handleCancel = useCallback(() => {
        handleClear();
        onClose();
    }, [handleClear, onClose]);

    // handleSubmit marks invalid fields red, then validates the values against
    // their rules and the running job (FE-02) and configures the job; the
    // messages say why the values were rejected or why the configure failed.
    const handleSubmit = async (e: React.FormEvent) => {
        e.preventDefault();
        simulationParameters.forEach(row =>
            validateSimulationParameters(false, row.key, row.value, simulationParametersValidationErrors));
        setErrors(await submitRuntimeParameters(simulationParameters, dashboardState.getRuntimeParameters(), onSubmit));
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
                        {errors.length > 0 && <ErrorList errors={errors}/>}
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