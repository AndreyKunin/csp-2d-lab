import React, {useRef} from 'react';
import {UnitType} from "./InputForm";

interface LoadFormButtonProps {
    text: string;
    onDataLoaded?: (
        sheets: string[][] | null,
        parts: string[][],
        sawWidth: string | null,
        units: UnitType | null,
        parameters: Map<string, string> | null) => void;
    className?: string;
}

export const LoadFormButton: React.FC<LoadFormButtonProps> = ({ text, className, onDataLoaded }) => {
    const   fileInputRef = useRef<HTMLInputElement>(null);

    const handleFileUpload = (event: React.ChangeEvent<HTMLInputElement>) => {
        const file = event.target.files?.[0];
        if (!file) {
            return;
        }

        const reader = new FileReader();

        reader.onload = (e) => {
            const text = e.target?.result as string;
            if (!text) {
                return;
            }

            const lines = text.trim().split('\n');

            type ParserState = 'idle' | 'sheets' | 'parts' | 'saw_width' | 'units' | 'parameters';
            let state: ParserState = 'idle';
            let parsedSheets: string[][] | null = null;
            let parsedParts: string[][] = [];
            let parsedParameters: Map<string, string> | null = null;
            let sawWidth: string | null = null;
            let units: UnitType | null = null;

            for (const line of lines) {
                const trimmedLine = line.trim();

                if (trimmedLine === 'Sheets') {
                    state = 'sheets';
                    parsedSheets = [];
                } else if (trimmedLine === 'Parts') {
                    state = 'parts';
                } else if (trimmedLine === 'Parameters') {
                    state = 'parameters';
                    parsedParameters = new Map();
                } else if (trimmedLine === 'SawWidth') {
                    state = 'saw_width';
                } else if (trimmedLine === 'Units') {
                    state = 'units';
                } else if (trimmedLine === '') {
                    state = 'idle';
                } else {
                    if (state === 'sheets') {
                        if (parsedSheets === null) {
                            parsedSheets = [];
                        }
                        parsedSheets.push(trimmedLine.split(';'));
                    } else if (state === 'parts') {
                        parsedParts.push(trimmedLine.split(';'));
                    } else if (state === 'saw_width') {
                        sawWidth = trimmedLine;
                    } else if (state === 'units') {
                        units = trimmedLine === 'mm' ? 'mm' : 'in';
                    } else if (state === 'parameters') {
                        if (parsedParameters) {
                            const colonIndex = trimmedLine.indexOf(':');
                            if (colonIndex !== -1) {
                                const key = trimmedLine.substring(0, colonIndex).trim();
                                const value = trimmedLine.substring(colonIndex + 1).trim();
                                parsedParameters.set(key, value);
                            }
                        }
                    }
                }
            }

            if (fileInputRef.current) {
                fileInputRef.current.value = '';
            }

            if (onDataLoaded) {
                onDataLoaded(parsedSheets, parsedParts, sawWidth, units, parsedParameters);
            }
        };

        reader.readAsText(file);
    };

    return (
        <>
            <input
                type="file"
                accept=".txt"
                onChange={handleFileUpload}
                style={{ display: 'none' }}
                ref={fileInputRef}
            />
            <div onClick={() => fileInputRef.current?.click()} className={className}>{text}</div>
        </>
    );
};

export default LoadFormButton;