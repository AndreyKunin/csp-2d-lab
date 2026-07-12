import React, {useCallback} from 'react';
import {RadioGroup} from './RadioGroup';

export type SawWidthOption = {
    id: string;
    text: string;
    in: string;
    mm: string;
};

interface SawWidthRadioGroupProps {
    id: string;
    options: SawWidthOption[];
    sawWidth: string;
    onSawWidthChange: (value: string) => void;
    onOptionSelected: (option: SawWidthOption | null) => void;
    selectedId: string;
    validationError?: boolean;
}

export const SawWidthRadioGroup: React.FC<SawWidthRadioGroupProps> = ({
                                                                          id,
                                                                          options,
                                                                          sawWidth,
                                                                          onSawWidthChange,
                                                                          onOptionSelected,
                                                                          selectedId,
                                                                          validationError,
                                                                      }) => {
    const radioOptions = options.map(o => ({ id: o.id, value: o.text }));

    const handleRadioChange = useCallback((optionId: string) => {
        const option = options.find(o => o.id === optionId) ?? null;
        onOptionSelected(option);
    }, [options, onOptionSelected]);

    const handleInputChange = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
        onOptionSelected(null);
        onSawWidthChange(e.target.value);
    }, [onOptionSelected, onSawWidthChange]);

    return (
        <div className="sawWidthRadioGroup">
            <RadioGroup
                id={id}
                options={radioOptions}
                selectedId={selectedId}
                type="horizontal"
                onChange={handleRadioChange}
                allowDeselect={true}
            />
            <input
                type="text"
                value={sawWidth}
                onChange={handleInputChange}
                className={validationError ? 'invalidInput' : ''}
            />
        </div>
    );
};