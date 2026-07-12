import React from 'react';

interface ThreePhaseKnobProps {
    values: number[];
    index: number;
    title?: string;
    altText?: string;
    clickHandler: (value: number) => void;
}


export const ThreePhaseKnob: React.FC<ThreePhaseKnobProps> = ({ values, index, title, altText, clickHandler }) => {
    return (
        <div className="threePhaseKnob">
            <div>
                {values[1]}
            </div>
            <div>
                <div>{values[0]}</div>
                <div>
                    <img
                        src={`/lab/switch-${index}.svg`}
                        alt={altText}
                        width="64px"
                        height="auto"
                        onClick={() => clickHandler(values[(index + 1) % 3])}
                    />
                </div>
                <div>{values[2]}</div>
            </div>
            <div>{title}</div>
        </div>
    );
};