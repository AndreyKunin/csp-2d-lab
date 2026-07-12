import React from 'react';

interface InputFieldProps {
    id: string;
    label: string;
    value: string;
    validationError?: boolean;
    update: (value: string) => void;
}

export const InputField: React.FC<InputFieldProps> = (
    ({ id, label, value, validationError, update }: InputFieldProps) => {
        return (
            <div className="inputField" id={id}>
                <div>{label}</div>
                <div>
                    <input
                        type="text"
                        value={value}
                        onChange={(e) => update(e.target.value)}
                        className={validationError ? 'invalidInput' : ''}
                    />
                </div>
                <div></div>
                <div></div>
            </div>
        );
    });