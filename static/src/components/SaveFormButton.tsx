import React from 'react';
import {PropertyItem} from "../utils/parameters";
import {UnitType} from "./InputForm";

interface FileSaveButtonProps {
    text: string;
    sheetsData: string[][];
    partsData: string[][];
    parameters?: PropertyItem[];
    sawWidth?: string;
    units?: UnitType;
    filename?: string;
    className?: string;
}

export const SaveFormButton: React.FC<FileSaveButtonProps> = (
    {text, sheetsData, partsData, parameters, sawWidth, units, className, filename = 'Form.txt'}
) => {
    const handleSave = async () => {
        let content = '';

        if (sheetsData.length > 0 && !(sheetsData.length === 1 && sheetsData[0].every(cell => cell === ''))) {
            content += 'Sheets\n';
            sheetsData.forEach(row => {
                content += row.join(';') + '\n';
            });
            content += '\n';
        }

        if (partsData.length > 0) {
            content += 'Parts\n';
            partsData.forEach(row => {
                content += row.join(';') + '\n';
            });
        }

        if (sawWidth !== undefined && sawWidth !== '') {
            content += '\n';
            content += 'SawWidth\n';
            content += `${sawWidth}\n`;
        }

        if (units !== undefined) {
            content += '\n';
            content += 'Units\n';
            content += `${units}\n`;
        }

        if (parameters && parameters.length > 0) {
            content += '\n';
            content += 'Parameters\n';
            parameters.forEach(param => {
                content += `${param.key}: ${param.value}\n`;
            });
        }

        const blob = new Blob([content], { type: 'text/plain' });

        // Check if File System Access API is supported
        if ('showSaveFilePicker' in window) {
            try {
                const handle = await (window as any).showSaveFilePicker({
                    suggestedName: filename,
                    types: [
                        {
                            description: 'Text Files',
                            accept: {
                                'text/plain': ['.txt']
                            }
                        }
                    ]
                });

                const writable = await handle.createWritable();
                await writable.write(blob);
                await writable.close();
            } catch (err: any) {
                // User canceled the picker or an error occurred
                if (err.name !== 'AbortError') {
                    console.error('Error saving file:', err);
                    // Fallback to download
                    downloadFile(blob, filename);
                }
            }
        } else {
            // Fallback for browsers that don't support File System Access API
            downloadFile(blob, filename);
        }
    };

    const downloadFile = (blob: Blob, filename: string) => {
        const url = URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.href = url;
        link.download = filename;
        link.click();
        URL.revokeObjectURL(url);
    };

    return (
        <div onClick={handleSave} className={className}>{text}</div>
    );
};

export default SaveFormButton;