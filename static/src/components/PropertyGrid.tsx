import React, {useMemo} from 'react';
import {Tooltip} from "./Tooltip";
import {PropertyItem} from "../utils/parameters";

interface PropertyGridProps {
    id: string;
    data: PropertyItem[];
    validationErrors: Map<string, boolean>;
    update: (rowIndex: number, value: string) => void;
}


export const PropertyGrid: React.FC<PropertyGridProps> = (
    ({ id, data, validationErrors, update }: PropertyGridProps) => {

        const gridLength = useMemo(() => data.length, [data]);

        return (
            <div className="propertyGrid" id={id}>
                {data.map((row, rowIndex) => (
                    <div key={row.key}>
                        <div key={row.key+"-name"}><input type="text" disabled={true} value={row.displayName}/></div>
                        <div key={row.key+"-value"}>
                            <input
                                key={row.key+"-input"}
                                type="text"
                                value={row.value}
                                onChange={(e) => update(rowIndex, e.target.value)}
                                className={validationErrors.get(row.key) ? 'invalidInput' : ''}
                            />
                        </div>
                        <div key={row.key+"-actions"}>
                            {!!row.tooltip && (
                                <Tooltip
                                    id={row.key}
                                    displayOnTop={rowIndex > gridLength / 2 - 1}
                                    text={row.tooltip || ''}
                                />
                            )}
                        </div>
                    </div>
                ))}
            </div>
        );
    });