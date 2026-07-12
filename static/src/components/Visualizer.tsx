import React, {useCallback, useEffect, useMemo, useRef} from 'react';
import {Model} from "../utils/model";
import {getTree} from "../utils/rect";
import {Layout} from "../utils/layout";
import UpButton from "./UpButton";
import SavePdfButton from "./SavePdfButton";
import {generatePdf} from "../services/requests";
import {parseSize} from "../utils/numbers";

interface VisualizerProps {
    model: Model | null;
    selectedSolution: number | null;
    setErrors: (errors: string[]) => void;
}

export const Visualizer: React.FC<VisualizerProps> = ({ model, selectedSolution, setErrors }) => {
    const renderedCanvasesRef = useRef<HTMLCanvasElement[]>([]);

    const drawSheetLayout = useCallback((selectedSheet: number) => {
        const canvas = document.getElementById(`display_area_${selectedSheet}`) as HTMLCanvasElement;
        if (canvas && model && selectedSolution !== null && selectedSheet !== null) {
            try {
                const gap = parseSize(model.sawWidth, model.fraction);
                const rect = getTree(model, selectedSolution, selectedSheet, gap);
                const sheet = model.getNumericSheets(true);
                const layout = new Layout(canvas, model.units, sheet?.width as number, sheet?.length as number);
                layout.resizeToViewport();
                layout.drawLayout(rect, selectedSheet);
            } catch (error) {
                console.error('Failed to draw layout:', error);
                const ctx = canvas.getContext('2d');
                if (ctx) {
                    ctx.clearRect(0, 0, canvas.width, canvas.height);
                }
            }
        }
    }, [model, selectedSolution]);

    const sheets = useMemo(() =>
            model && model.solutions && selectedSolution !== null ? model.solutions[selectedSolution].sheets : []
        , [model, selectedSolution]
    );

    const handleSavePdf = useCallback(async () => {
        const canvases = renderedCanvasesRef.current;
        if (!model || canvases.length === 0) {
            // No-op: results are not ready yet.
            return;
        }
        try {
            const blob = await generatePdf(model, canvases);
            const url = URL.createObjectURL(blob);
            window.open(url, '_blank');
            setTimeout(() => URL.revokeObjectURL(url), 60_000);
        } catch (e) {
            setErrors([(e as Error).message]);
        }
    }, [model]);

    useEffect(() => {
        sheets.forEach((sheet) => drawSheetLayout(sheet.number));
        renderedCanvasesRef.current = sheets
            .map(s => document.getElementById(`display_area_${s.number}`) as HTMLCanvasElement | null)
            .filter((c): c is HTMLCanvasElement => c !== null);
    }, [sheets]);

    if (!model || !model.solutions) {
        return <div></div>;
    }

    return (
      <div>
        {sheets.map((sheet, index) => (
            <div className="visualizer" key={"da"+index}>
                <div>
                    <div></div>
                </div>
                <div>
                    <div></div>
                    <div style={{ fontFamily: 'blueprint, monospace' }}>
                        <canvas id={"display_area_"+sheet.number}></canvas>
                    </div>
                </div>
            </div>
        ))}
        <div className="visualizerFooter">
            <div className="actions">
                <SavePdfButton
                    onClick={handleSavePdf}
                    className={'fileButton stickyButton saveButton'}
                    text="Save PDF"
                />
                <UpButton className="fileButton stickyButton" />
            </div>
        </div>
      </div>
    );
};