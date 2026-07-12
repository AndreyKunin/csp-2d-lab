import React, {useCallback} from 'react';

interface TooltipProps {
    id: string;
    displayOnTop: boolean;
    text: string;
}


export const Tooltip: React.FC<TooltipProps> = (
    ({ id, displayOnTop, text }: TooltipProps) => {

        const showTooltip = useCallback((key: string) => {
            const tooltip = document.getElementById(key+"-tooltip");
            if (tooltip) {
                tooltip.classList.toggle('visible');
            }
        }, [id]);

        const hideTooltip = useCallback((key: string) => {
            const tooltip = document.getElementById(key+"-tooltip");
            if (tooltip) {
                tooltip.classList.remove('visible');
            }
        }, [id]);

        return (
            <div
                className="tooltip-container"
                onClick={() => showTooltip(id)}
                onMouseOut={() => hideTooltip(id)}
            >
                <span>?</span>
                <span
                    id={id+"-tooltip"}
                    className={"book-tooltip-content " + (displayOnTop ? "top" : "bottom")}
                >{text}</span>
            </div>
        );
    });