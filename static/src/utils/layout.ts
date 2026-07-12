import {RectNode} from './rect';
import * as math from 'mathjs';
import {Fraction} from "mathjs";

interface RectView {
    x: number;
    y: number;
    w: number;
    h: number;
    slice: Slice;
}

interface Slice {
    x: number;
    y: number;
    direction: string;
    rect: RectNode;
    getOppositeDirection(): string;
}

export class Layout {
    canvas: HTMLCanvasElement;
    sheetWidth?: number;
    sheetLength?: number;
    font: string;
    fontSize: number;
    sliceRects: RectView[];
    partRects: RectView[];
    gapRects: { x: number; y: number; w: number; h: number }[];
    units: string;

    private readonly MAX_SIZE_RATIO = 0.95; // 95%
    private readonly SHIFT = 19; // 19px is about 5mm in Chrome and FF

    constructor(canvas: HTMLCanvasElement, units: string, sheetWidth?: number, sheetLength?: number) {
        this.canvas = canvas;
        this.sheetWidth = sheetWidth;
        this.sheetLength = sheetLength;
        this.units = units;
        this.font = 'blueprint';
        this.fontSize = 16;
        this.sliceRects = [];
        this.partRects = [];
        this.gapRects = [];
    }

    clear(): void {
        const ctx = this.canvas.getContext('2d');
        if (ctx) {
            ctx.clearRect(0, 0, this.canvas.width, this.canvas.height);
        }
    }

    drawLayout(rect: RectNode, sheetNumber: number): void {
        const ctx = this.canvas.getContext('2d');
        if (!ctx) return;

        ctx.lineWidth = 1;
        ctx.strokeStyle = "#000000";

        const scale = this.rescale(rect); // model-units -> pixels factor
        ctx.clearRect(0, 0, this.canvas.width, this.canvas.height);

        this.sliceRects = []; // faux/waste + operation rects (dashed)
        this.partRects = []; // real parts (solid)
        const stack: Slice[] = [];

        // Root starts at (SHIFT, SHIFT) with horizontal orientation.
        stack.push(this.createSlice(this.SHIFT, this.SHIFT, 'H', rect));

        while (stack.length > 0) {
            const slice = stack.pop();
            if (!slice) break;

            // Node size in pixels. NOTE: getHeight()=smaller dim, getWidth()=larger dimension.
            const heightPx = slice.rect.getHeight() * scale;
            const widthPx = slice.rect.getWidth() * scale;
            const isVertical = slice.direction === 'V';

            if (slice.rect.isOperation) {
                let x0: number, y0: number, d1: string, d2: string;
                let sliceOnScreenWidthPx = isVertical ? heightPx : widthPx;
                let sliceOnScreenHeightPx = isVertical ? widthPx : heightPx;
                // Left child's dimensions in px, used as the offset to place the right child.
                const leftChildHeightPx = slice.rect.left!.getHeight() * scale;
                const leftChildWidthPx = slice.rect.left!.getWidth() * scale;

                const gap = slice.rect.getGap();
                const gapPx = gap * scale; // gap size in pixels


                if (slice.rect.isLongSlice()) {
                    // Do children keep the parent's orientation, or are they rotated?
                    const sameDirection = math.equal(slice.rect.width, math.max(slice.rect.left!.width, slice.rect.right!.width));
                    d1 = d2 = sameDirection ? slice.direction : slice.getOppositeDirection();

                    // Compute the right child's top-left. The offset (leftChildHeightPx)
                    // is exactly where the gap should be inserted.
                    if (isVertical) {
                        if (sameDirection) {
                            x0 = slice.x + leftChildHeightPx + gapPx;
                            y0 = slice.y;
                        } else {
                            x0 = slice.x;
                            y0 = slice.y + leftChildHeightPx + gapPx;
                        }
                    } else {
                        if (sameDirection) {
                            x0 = slice.x;
                            y0 = slice.y + leftChildHeightPx + gapPx;
                        } else {
                            x0 = slice.x + leftChildHeightPx + gapPx;
                            y0 = slice.y;
                        }
                    }
                } else if (slice.rect.isShortSlice()) {
                    // Short slice may flip one child so touching edges match.
                    const widthIfLeftFlipped = math.add(math.add(slice.rect.left!.height, slice.rect.right!.width), gap);
                    const widthIfRightFlipped = math.add(math.add(slice.rect.left!.width, slice.rect.right!.height), gap);
                    const cannotLeftFlip = math.larger(slice.rect.left!.width, slice.rect.height);
                    const cannotRightFlip = math.larger(slice.rect.right!.width, slice.rect.height);
                    const leftChildFlipped = math.equal(slice.rect.width, widthIfLeftFlipped) && !cannotLeftFlip;
                    const rightChildFlipped = math.equal(slice.rect.width, widthIfRightFlipped) && !cannotRightFlip;
                    d1 = leftChildFlipped ? slice.getOppositeDirection() : slice.direction;
                    d2 = rightChildFlipped ? slice.getOppositeDirection() : slice.direction;

                    if (isVertical) {
                        if (leftChildFlipped) {
                            x0 = slice.x;
                            y0 = slice.y + leftChildHeightPx + gapPx;
                        } else {
                            x0 = slice.x;
                            y0 = slice.y + leftChildWidthPx + gapPx;
                        }
                    } else {
                        if (leftChildFlipped) {
                            x0 = slice.x + leftChildHeightPx + gapPx;
                            y0 = slice.y;
                        } else {
                            x0 = slice.x + leftChildWidthPx + gapPx;
                            y0 = slice.y;
                        }
                    }
                } else {
                    // Default case
                    x0 = slice.x;
                    y0 = slice.y;
                    d1 = d2 = slice.direction;
                }

                // Operation node itself is recorded as a slice rect (its bounding box).
                const rectView = this.createRectView(slice.x, slice.y, sliceOnScreenWidthPx, sliceOnScreenHeightPx, slice);
                this.sliceRects.push(rectView);

                // Push children. Left keeps the parent's origin; right is offset by x0/y0.
                stack.push(this.createSlice(x0, y0, d2, slice.rect.right!));
                stack.push(this.createSlice(slice.x, slice.y, d1, slice.rect.left!));
            } else {
                // Leaf: real part or faux waste.
                const rectView = isVertical
                    ? this.createRectView(slice.x, slice.y, heightPx, widthPx, slice)
                    : this.createRectView(slice.x, slice.y, widthPx, heightPx, slice);

                if (slice.rect.isFaux) {
                    this.sliceRects.push(rectView);
                } else {
                    this.partRects.push(rectView);
                }
            }
        }

        // Draw faux/slice rects (dashed), then parts (solid), then labels/sizes.
        this.sliceRects.forEach(r => {
            const drawSize = r.slice.rect.isFaux && scale * rect.getMinSize() > this.SHIFT;
            this.drawRect(ctx, r.x, r.y, r.w, r.h, [5, 5]);
            if (drawSize) {
                this.drawSize(ctx, r, this.units);
            }
        });
        this.partRects.forEach(r => this.drawPartRect(ctx, r, this.units));
        if (this.sliceRects[0]) {
            this.drawTotalSize(ctx, this.sliceRects[0], this.units);
        }

        this.drawSheetNumber(ctx, sheetNumber + 1, this.canvas.width - 10, this.canvas.height - 10);
    }

    resizeToViewport(): void {
        const { width, height } = this.canvas.getBoundingClientRect();
        this.scaleCanvas(this.canvas, 'w', width);
        this.scaleCanvas(this.canvas, 'h', height);
    }

    private rescale(rect: RectNode): number {
        const { width, height } = this.canvas.getBoundingClientRect();
        const widthPx = Math.floor(width * this.MAX_SIZE_RATIO) - this.SHIFT;
        const heightPx = Math.floor(height * this.MAX_SIZE_RATIO) - this.SHIFT;
        const contentWidth = this.sheetWidth ? this.sheetWidth : rect.getWidth();
        const contentHeight = this.sheetLength ? this.sheetLength : rect.getHeight();
        const scaleX = widthPx / contentWidth;
        const scaleY = heightPx / contentHeight;

        return Math.min(scaleX, scaleY);
    }

    private drawPartRect(ctx: CanvasRenderingContext2D, rect: RectView, units: string): void {
        this.drawRect(ctx, rect.x, rect.y, rect.w, rect.h, []);
        this.drawLabel(ctx, rect);
        this.drawSize(ctx, rect, units);
    }

    private drawLabel(ctx: CanvasRenderingContext2D, rect: RectView): void {
        ctx.font = `${Math.min(this.fontSize, rect.h - 4)}px ${this.font}`;
        ctx.fillStyle = "#5703EF";
        ctx.textAlign = "left";
        ctx.textBaseline = "top";
        ctx.fillText(rect.slice.rect.name, rect.x + 2, rect.y + 2, rect.w - 4);
    }

    private drawSheetNumber(ctx: CanvasRenderingContext2D, n: number, w: number, h: number): void {
        ctx.font = `${this.fontSize * 1.25}px ${this.font}`;
        ctx.fillStyle = "#5703EF";
        ctx.textAlign = "right";
        ctx.textBaseline = "bottom";
        ctx.fillText("Sheet " + n, w, h, 250);
    }

    private drawSize(ctx: CanvasRenderingContext2D, rect: RectView, units: string): void {
        ctx.font = `${Math.min(this.fontSize / 1.2, rect.h - 4)}px ${this.font}`;
        ctx.textAlign = "center";
        ctx.textBaseline = "middle";
        ctx.fillStyle = "#5703EF";
        const sizeLabel = `${rect.slice.rect.formatDimensions()}${units === 'in' ? '"' : ' mm'}`;
        ctx.fillText(sizeLabel, rect.x + rect.w / 2, rect.y + rect.h / 2, rect.w - 4);
    }

    private drawTotalSize(ctx: CanvasRenderingContext2D, rect: RectView, units: string): void {
        ctx.font = `${this.fontSize / 1.2}px ${this.font}`;
        ctx.fillStyle = "#5703EF";
        ctx.textAlign = "center";
        ctx.textBaseline = "top";
        const widthLabel = `${rect.slice.rect.formatWidth()}${units === 'in' ? '"' : ' mm'}`;
        ctx.fillText(widthLabel, rect.x + rect.w / 2, rect.y + rect.h + 4, rect.w - 4);
        ctx.textAlign = "left";
        ctx.textBaseline = "middle";
        const heightLabel = `${rect.slice.rect.formatHeight()}${units === 'in' ? '"' : ' mm'}`;
        ctx.fillText(heightLabel, rect.x + rect.w + 4, rect.y + rect.h / 2, rect.h - 4);
    }

    private drawRect(ctx: CanvasRenderingContext2D, x: number, y: number, w: number, h: number, dash: number[]): void {
        ctx.clearRect(x - 1, y - 1, w + 2, h + 2);
        ctx.setLineDash(dash);
        ctx.beginPath();
        ctx.rect(x, y, w, h);
        ctx.stroke();
    }

    private createRectView(x: number, y: number, w: number, h: number, slice: Slice): RectView {
        return { x, y, w, h, slice };
    }

    private createSlice(x: number, y: number, direction: string, rect: RectNode): Slice {
        return {
            x,
            y,
            direction,
            rect,
            getOppositeDirection: () => direction === 'H' ? 'V' : 'H'
        };
    }

    private scaleCanvas(canvas: HTMLCanvasElement, direction: string, newValue: number): void {
        if (direction === 'h') {
            canvas.height = newValue;
        } else {
            canvas.width = newValue;
        }
    }
}

/**
 * Converts a single HTMLCanvasElement to a PNG Blob (preserves transparency).
 */
export function canvasToPngBlob(
    canvas: HTMLCanvasElement
): Promise<Blob> {
    return new Promise((resolve, reject) => {
        canvas.toBlob(
            (blob) => {
                if (blob) {
                    resolve(blob);
                } else {
                    reject(new Error("Canvas toBlob() returned null"));
                }
            },
            "image/png"
        );
    });
}

/**
 * Converts multiple canvases to PNG blobs in parallel.
 */
export async function canvasesToPngBlobs(
    canvases: HTMLCanvasElement[]
): Promise<Blob[]> {
    return Promise.all(canvases.map((c) => canvasToPngBlob(c)));
}