import React from 'react';

interface FixedGridProps {
    id: string;
    headers: string[];
    data: string[][];
    visible: boolean;
    validationErrors: boolean[][];
    update: (rowIndex: number, colIndex: number, value: string) => void;
}

export const FixedGrid: React.FC<FixedGridProps> = (
    ({ id, headers, data, visible, validationErrors, update }: FixedGridProps) => {

    return (
        <div className={`fixedGrid ${visible ? 'visible' : 'hidden'}`} id={id}>
            <div>
                {headers.map((header, index) => (
                    <div key={id+"h"+index}>{header}</div>
                ))}
            </div>
            {data.map((row, rowIndex) => (
                <div key={id+rowIndex}>
                    {row.map((cell, colIndex) => (
                        <div key={id+rowIndex+"c"+colIndex}>
                            <input
                                key={colIndex}
                                type="text"
                                value={cell}
                                onChange={(e) => update(rowIndex, colIndex, e.target.value)}
                                className={validationErrors[rowIndex]?.[colIndex] ? 'invalidInput' : ''}
                            />
                        </div>
                    ))}
                </div>
            ))}
        </div>
    );
});