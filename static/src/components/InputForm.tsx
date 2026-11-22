import React, {MutableRefObject, useCallback, useEffect, useMemo, useRef, useState} from 'react';
import {DynamicGrid} from "./DynamicGrid";
import {RadioGroup} from "./RadioGroup";
import {FixedGrid} from "./FixedGrid";
import {Checkbox} from "./Checkbox";
import {PropertyGrid} from "./PropertyGrid";
import {getDefaultParameters} from "../services/requests";
import {SimulationParameters} from "../utils/simulation";
import {
    getInitialParameters,
    PropertyItem,
    validateSimulationParameters,
    validateCount,
    validateSize,
    validateFraction
} from "../utils/parameters";
import LoadFormButton from "./LoadFormButton";
import SaveFormButton from "./SaveFormButton";
import {SawWidthOption, SawWidthRadioGroup} from "./SawWidthRadioGroup";
import {isFraction} from "../utils/numbers";
import {overriddenParameters} from "../utils/parameterDefinitions";


const enum PartsRowType {
    Width,
    Length,
    Count,
}

const enum SheetsRowType {
    Width,
    Length,
}

export type InputFormData = {
    unitType: UnitType;
    partsData: string[][];
    sheetsData?: string[][];
    sawWidth: string;
    parameters?: PropertyItem[];
}

type SheetType = "unlimited_sheet" | "limited_sheet";
export type UnitType = "mm" | "in";

interface InputFormProps {
    isLoading: boolean;
    onSubmit: (formData: InputFormData) => void;
    onError: (messages: string[]) => void;
    onReset: () => void;
}

export const InputForm: React.FC<InputFormProps> = ({isLoading, onSubmit, onError, onReset}: InputFormProps) => {
    const [partsData, setPartsData] = useState<string[][]>([]);
    const partsValidationErrors = useRef<boolean[][]>([]);
    const [sheetsData, setSheetsData] = useState<string[][]>([['', '']]);
    const sheetsValidationErrors = useRef<boolean[][]>([]);
    const [sheetType, setSheetType] = useState<SheetType>("limited_sheet");
    const [unitType, setUnitType] = useState<UnitType>('in');
    const [sawWidth, setSawWidth] = useState<string>('0');
    const [sawWidthInvalid, setSawWidthInvalid] = useState<boolean>(false);
    const [sawWidthSelectedId, setSawWidthSelectedId] = useState<string>('z');
    const sawWidthSelectedIdRef = useRef<string>('z');

    const [defaultParameters, setDefaultParameters] = useState<SimulationParameters>();
    const [simulationParameters, setSimulationParameters] = useState<PropertyItem[]>([]);
    const [useCustomParameters, setUseCustomParameters] = useState<boolean>(false);
    const simulationParametersValidationErrors = useRef<Map<string, boolean>>(new Map());

    // Parameters the server overrides for this number of parts (sum of the
    // counts) are shown disabled and empty and are not sent; when a change of
    // the parts releases one, it gets its default back.
    const partsCount = useMemo(() => partsData.reduce((sum, row) => {
        const count = Number(row[2]);
        return sum + (Number.isInteger(count) && count > 0 ? count : 0);
    }, 0), [partsData]);
    const overridden = useMemo(() => overriddenParameters(partsCount), [partsCount]);
    const previousOverridden = useRef<Set<string>>(new Set());
    useEffect(() => {
        const released = [...previousOverridden.current].filter(key => !overridden.has(key));
        previousOverridden.current = overridden;
        overridden.forEach(key => simulationParametersValidationErrors.current.set(key, false));
        if (released.length === 0 || !defaultParameters) {
            return;
        }
        const defaults = new Map(getInitialParameters(defaultParameters).map(item => [item.key, item.value]));
        setSimulationParameters(items => items.map(item =>
            released.includes(item.key) ? {...item, value: defaults.get(item.key) ?? ''} : item
        ));
    }, [overridden, defaultParameters]);
    const displayedParameters = useMemo(() => simulationParameters.map(item =>
        overridden.has(item.key) ? {...item, disabled: true} : item
    ), [simulationParameters, overridden]);
    const sentParameters = useMemo(() => simulationParameters.filter(item => !overridden.has(item.key)),
        [simulationParameters, overridden]);

    const sawWidthOptions = useMemo(() => [
        {id: "z" , text: "Zero"     , in: "0"   , mm: "0"  },
        {id: "tk", text: "Thin kerf", in: "3/32", mm: "2.4"},
        {id: "fk", text: "Full kerf", in: "1/8" , mm: "3.2"}
    ], []);

    useEffect(() => {
        getDefaultParameters().then(defaultParameters => {
            setDefaultParameters(defaultParameters);
            setSimulationParameters(getInitialParameters(defaultParameters));
        });
    }, []);

    const loadFromFile = useCallback((
        sheets: string[][] | null,
        parts: string[][],
        sawWidth: string | null,
        units: UnitType | null,
        parameters: Map<string, string> | null
    ) => {
        if (isLoading) {
            return;
        }
        setPartsData(parts);
        if (sheets) {
            setSheetsData(sheets);
            setSheetType("limited_sheet");
        } else {
            setSheetType("unlimited_sheet");
        }

        if (sawWidth) {
            setSawWidth(sawWidth);
            const matchingOption = sawWidthOptions.find(
                o => o.in === sawWidth || o.mm === sawWidth
            );
            const newId = matchingOption?.id ?? '';
            setSawWidthSelectedId(newId);
            sawWidthSelectedIdRef.current = newId;
        }

        if (units) {
            setUnitType(units);
        }

        if (parameters && parameters.size > 0) {
            setUseCustomParameters(true);
            const updatedParams = simulationParameters.map(param => {
                const loadedValue = parameters.get(param.key);
                if (loadedValue !== undefined) {
                    return { ...param, value: loadedValue };
                }
                return param;
            });
            setSimulationParameters(updatedParams);
        }
    }, [simulationParameters]);

    const clearProblemData = () => {
        if (isLoading) {
            return;
        }
        setPartsData([]);
        partsValidationErrors.current = [];
        setSheetsData([['', '']]);
        sheetsValidationErrors.current = [];
        setSheetType("limited_sheet");
        setSawWidth('0');
        setSawWidthInvalid(false);
        setSawWidthSelectedId('z');
        sawWidthSelectedIdRef.current = 'z';
        setUnitType('in');

        setUseCustomParameters(false);
        if (defaultParameters) {
            setSimulationParameters(getInitialParameters(defaultParameters));
        }
        simulationParametersValidationErrors.current = new Map();
        onReset();
    };

    const onFormSubmit = () => {
        if (isLoading) {
            return;
        }
        const formData = extractFormData();
        if (formData) {
            onSubmit(formData);
        } else {
            if (!partsData.length) {
                onError(["Please fill the parts table. Also, look at the tables above (if marked red)."]);
            } else {
                onError(["Please correct numbers in the tables above (marked red)."]);
            }
        }
    }

    const updatePart = useCallback((rowIndex: number, colIndex: number, value: string) => {
        const newRows = [...partsData];
        newRows[rowIndex][colIndex] = value;
        setPartsData(newRows);
        validateParts(true, rowIndex, colIndex, value);
    }, [partsData]);

    const addPart = useCallback(() => {
        setPartsData([...partsData, ['', '', '']]);
    }, [partsData]);

    const removePart = useCallback((index: number) => {
        const newRows = partsData.filter((_, i) => i !== index);
        setPartsData(newRows);
    }, [partsData]);

    const updateSheet = useCallback((rowIndex: number, colIndex: number, value: string) => {
        const newRows = [...sheetsData];
        newRows[rowIndex][colIndex] = value;
        setSheetsData(newRows);
        validateSheets(true, rowIndex, colIndex, value);
    }, [sheetsData]);

    const updateSawWidth = useCallback((value: string) => {
        setSawWidth(value);
        validateSawWidth(value);
    }, []);

    const handleSawWidthOptionSelected = useCallback((option: SawWidthOption | null) => {
        if (option) {
            setSawWidthSelectedId(option.id);
            sawWidthSelectedIdRef.current = option.id;
            const value = unitType === 'mm' ? option.mm : option.in;
            setSawWidth(value);
            validateSawWidth(value);
        } else {
            setSawWidthSelectedId('');
            sawWidthSelectedIdRef.current = '';
        }
    }, [unitType]);

    const handleUnitTypeChange = useCallback((newUnitType: string) => {
        setUnitType(newUnitType as UnitType);
        const option = sawWidthOptions.find(o => o.id === sawWidthSelectedIdRef.current);
        if (option) {
            const value = newUnitType === 'mm' ? option.mm : option.in;
            setSawWidth(value);
            validateSawWidth(value);
        }
    }, [sawWidthOptions]);

    const updateSimulationParameters = useCallback((rowIndex: number, value: string) => {
        const newRows = [...simulationParameters];
        const key = newRows[rowIndex].key;
        newRows[rowIndex].value = value;
        setSimulationParameters(newRows);
        validateSimulationParameters(true, key, value, simulationParametersValidationErrors);
    }, [simulationParameters]);

    const extractFormData = (): InputFormData | null => {
        const partsResults = partsData.length > 0 && partsData.map((row, i) => {
            const cellResults = row.map((cell, j) => validateParts(false, i, j, cell));
            return cellResults.every(r => r);
        });
        const partsDataValid = partsResults && partsResults.every(r => r);

        let sheetDataValid = true;
        const isLimitedSheet = sheetType === "limited_sheet";
        if (isLimitedSheet) {
            if (sheetsData.length === 0) {
                sheetDataValid = false;
            } else {
                const sheetResults = sheetsData.map((row, i) => {
                    const cellResults = row.map((cell, j) => validateSheets(false, i, j, cell));
                    return cellResults.every(r => r);
                });
                sheetDataValid = sheetResults && sheetResults.every(r => r);
            }
        }

        let sawWidthValid = validateSawWidth(sawWidth);

        let parameterDataValid = true;
        if (useCustomParameters) {
            const parameterResults = sentParameters.map((row) => {
                return validateSimulationParameters(false, row.key, row.value, simulationParametersValidationErrors);
            });
            parameterDataValid = parameterResults && parameterResults.every(r => r);
        }

        if (!partsDataValid || !sheetDataValid || !sawWidthValid || !parameterDataValid) {
            return null;
        }

        return {
            unitType,
            partsData,
            sheetsData: isLimitedSheet ? sheetsData : undefined,
            sawWidth,
            parameters: useCustomParameters ? sentParameters : undefined,
        };
    };

    const validateSawWidth = (value: string): boolean => {
        if (!value) {
            setSawWidthInvalid(true);
            return false;
        } else {
            const fraction = isFraction(value);
            if (fraction && !validateFraction(value) || !fraction && (isNaN(Number(value)) || Number(value) < 0)) {
                setSawWidthInvalid(true);
                return false;
            }
        }
        setSawWidthInvalid(false);
        return true;
    }

    const validateParts = (
        allowEmpty: boolean, rowIndex: number, colIndex: number, value: string
    ): boolean => {
        if (!allowEmpty && !value) {
            setCellInvalid(rowIndex, colIndex, true, partsValidationErrors);
            return false;
        }
        let valid = false;
        switch (colIndex) {
            case PartsRowType.Width:
                valid = validateSize(value);
                break;
            case PartsRowType.Length:
                valid = validateSize(value);
                break;
            case PartsRowType.Count:
                valid = validateCount(value);
                break;
        }
        if (!valid) {
            setCellInvalid(rowIndex, colIndex, true, partsValidationErrors);
        } else {
            setCellInvalid(rowIndex, colIndex, false, partsValidationErrors);
        }
        return valid;
    }

    const validateSheets = (
        allowEmpty: boolean, rowIndex: number, colIndex: number, value: string
    ) => {
        if (!allowEmpty && !value) {
            setCellInvalid(rowIndex, colIndex, true, sheetsValidationErrors);
            return false;
        }
        let valid = false;
        switch (colIndex) {
            case SheetsRowType.Width:
                valid = validateSize(value);
                break;
            case SheetsRowType.Length:
                valid = validateSize(value);
                break;
        }
        if (!valid) {
            setCellInvalid(rowIndex, colIndex, true, sheetsValidationErrors);
        } else {
            setCellInvalid(rowIndex, colIndex, false, sheetsValidationErrors);
        }
        return valid;
    }

    const setCellInvalid = (
        rowIndex: number, colIndex: number, value: boolean, errorsRef: MutableRefObject<boolean[][]>
    )=> {
        if (!errorsRef.current[rowIndex]) {
            errorsRef.current[rowIndex] = [];
        }
        errorsRef.current[rowIndex][colIndex] = value;
    }

    return (
        <>
            <div className="formSection">
                <div><span className="sectionHeader">Parts</span></div>
                <DynamicGrid
                    id="parts_grid"
                    headers={["Width", "Length", "Count"]}
                    data={partsData}
                    validationErrors={partsValidationErrors.current}
                    update={updatePart}
                    addRow={addPart}
                    removeRow={removePart}
                />
                <div className="sectionSpacer">&nbsp;</div>

                <div><span className="sectionHeader">Sheets</span></div>
                <div className="twoColumns">
                    <RadioGroup
                        id="sheet_type"
                        options={[
                            {id: "unlimited_sheet", value: "Unlimited sheet"},
                            {id: "limited_sheet", value: "Set of sheets"}
                        ]}
                        selectedId={sheetType}
                        type={"vertical"}
                        onChange={v => setSheetType(v as SheetType)}
                    />
                    <FixedGrid
                        id="sheets_grid"
                        headers={["Width", "Length"]}
                        data={sheetsData}
                        visible={sheetType === "limited_sheet"}
                        validationErrors={sheetsValidationErrors.current}
                        update={updateSheet}
                    />
                </div>
                <div className="sectionSpacer">&nbsp;</div>

                <div><span className="sectionHeader">Saw width</span></div>
                <SawWidthRadioGroup
                    id="saw_width_selector"
                    options={sawWidthOptions}
                    sawWidth={sawWidth}
                    onSawWidthChange={updateSawWidth}
                    onOptionSelected={handleSawWidthOptionSelected}
                    selectedId={sawWidthSelectedId}
                    validationError={sawWidthInvalid}
                />
                <div className="sectionSpacer">&nbsp;</div>

                <div><span className="sectionHeader">Units</span></div>
                <RadioGroup
                    id="unit_type"
                    options={[
                        {id: "mm", value: "Metric (mm)"},
                        {id: "in", value: "Imperial (inch)"}
                    ]}
                    selectedId={unitType}
                    type={"horizontal"}
                    onChange={handleUnitTypeChange}
                />
                <div className="bottomLine"></div>
                <div className="sectionSpacer">&nbsp;</div>

                <div className="formSection actions">
                    <div
                        onClick={clearProblemData}
                        className={'clearButton' + (isLoading ? ' loading' : '')}>
                        Clear
                    </div>
                    <div
                        onClick={onFormSubmit}
                        className={'submitButton' + (isLoading ? ' loading' : '')}>
                        {isLoading ? 'Processing...' : 'Submit'}
                    </div>
                </div>
            </div>

            <div className="rightLine"/>

            <div className="formSection">
                <Checkbox
                    id="use_parameters"
                    checked={useCustomParameters}
                    label="Use custom simulation parameters (advanced)"
                    onChange={setUseCustomParameters}
                />

                {useCustomParameters && (
                    <>
                        <div className="sectionSpacer">&nbsp;</div>
                        <PropertyGrid
                            id="s_parameters"
                            data={displayedParameters}
                            validationErrors={simulationParametersValidationErrors.current}
                            update={updateSimulationParameters}
                        />
                        <div className="sectionSpacer">&nbsp;</div>
                    </>
                )}
                {!useCustomParameters && (
                    <>
                        <div className="bottomLine"></div>
                        <div className="sectionSpacer">&nbsp;</div>
                    </>
                )}

                <div className="formSection actions">
                    <SaveFormButton
                        partsData={partsData}
                        sheetsData={sheetsData}
                        parameters={useCustomParameters ? sentParameters : undefined}
                        sawWidth={sawWidth}
                        units={unitType}
                        text="Save"
                        className={'fileButton' + (isLoading ? ' loading' : '')}
                    />
                    <LoadFormButton
                        onDataLoaded={loadFromFile}
                        text="Load"
                        className={'fileButton' + (isLoading ? ' loading' : '')}
                    />
                </div>
            </div>

            <div className="formBreak"/>
        </>
    );
};