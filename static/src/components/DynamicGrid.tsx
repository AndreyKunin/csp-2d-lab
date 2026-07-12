import React from 'react';

interface DynamicGridProps {
    id: string;
    headers: string[];
    data: string[][];
    validationErrors: boolean[][];
    update: (rowIndex: number, colIndex: number, value: string) => void;
    addRow: () => void;
    removeRow: (index: number) => void;
}


export const DynamicGrid: React.FC<DynamicGridProps> = (
    ({ id, headers, data, validationErrors, update, addRow, removeRow }: DynamicGridProps) => {

    return (
        <div className="dynamicGrid" id={id}>
            <div key={id+"h"}>
                <div key={id+"num"}>#</div>
                {headers.map((header, index) => (
                    <div key={id+"h"+index}>{header}</div>
                ))}
                <div key={id+"h"+"actions"}>
                    <div>
                        <span onClick={addRow}>+</span>
                    </div>
                </div>
            </div>
            {data.map((row, rowIndex) => (
                <div key={id+rowIndex}>
                    <div key={id+rowIndex+"num"}><input type="text" disabled={true} value={`${(rowIndex + 1)}.`}/></div>
                    {row.map((cell, colIndex) => (
                        <div key={id+rowIndex+"c"+colIndex}>
                            <input
                                key={id+rowIndex+"i"+colIndex}
                                type="text"
                                value={cell}
                                onChange={(e) => update(rowIndex, colIndex, e.target.value)}
                                className={validationErrors[rowIndex]?.[colIndex] ? 'invalidInput' : ''}
                            />
                        </div>
                    ))}
                    <div key={id+rowIndex+"actions"}>
                        <div>
                            <span onClick={() => removeRow(rowIndex)}>-</span>
                        </div>
                    </div>
                </div>
            ))}
        </div>
    );
});