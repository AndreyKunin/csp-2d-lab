import React, {useCallback} from "react";

interface ToggleSwitchProps {
    isOn: boolean;
    altText: string;
    onClickHandler: (isOn: boolean) => void;
    className?: string;
    onText?: string;
    offText?: string;
}

export const ToggleSwitch: React.FC<ToggleSwitchProps> = ({ isOn, altText, className, onText, offText, onClickHandler }) => {
    const clickHandler = useCallback(() => {
        onClickHandler(!isOn);
    }, [isOn, onClickHandler])

    return (
        <div className={className}>
            <div className="toggleSwitch">
                <img
                    className={isOn ? 'down' : 'up'}
                    src={`/lab/toggle-${isOn ? 'down' : 'up'}.svg`}
                    onClick={clickHandler}
                    alt={altText}
                />
            </div>
            <div>
                <div>{offText}</div>
                <div>{onText}</div>
            </div>
        </div>
    )
}