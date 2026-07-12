import {Part, SheetData, Model} from './model';
import {parseSize, isFraction, formatSize} from './numbers';
import * as math from 'mathjs';
import {Fraction} from "mathjs";

export class RectNode {
    name: string;
    isFaux: boolean;
    isOperation: boolean;
    isFraction: boolean;
    width: Fraction | number;
    height: Fraction | number;
    minSize: Fraction | number;
    gap: Fraction | number;
    left?: RectNode;
    right?: RectNode;

    constructor(name: string, w: Fraction | number, h: Fraction | number, gap: Fraction | number, isFraction?: boolean) {
        this.name = name;
        this.isFaux = name[0] === '-';
        this.isOperation = !this.isFaux && isNaN(parseInt(name[0]));
        this.isFraction = isFraction || false;
        this.width = math.max(w, h);
        this.height = math.min(w, h);
        this.gap = gap;
        this.minSize = this.height;
        this.left = undefined;
        this.right = undefined;
    }

    getWidth(): number {
        return math.number(this.width);
    }

    getHeight(): number {
        return math.number(this.height);
    }

    getGap(): number {
        return math.number(this.gap);
    }

    getMinSize(): number {
        return math.number(this.minSize);
    }

    isShortSlice(): boolean {
        return this.name === 'S';
    }

    isLongSlice(): boolean {
        return this.name === 'L';
    }

    formatDimensions(): string {
        return `${formatSize(this.width, this.isFraction)} x ${formatSize(this.height, this.isFraction)}`;
    }

    formatWidth(): string {
        return formatSize(this.width, this.isFraction);
    }

    formatHeight(): string {
        return formatSize(this.height, this.isFraction);
    }

    joinRects(rect1: RectNode, rect2: RectNode, gap: Fraction | number): void {
        this.left = rect1;
        this.right = rect2;
        let h1 = rect1.height;
        let h2 = rect2.height;
        let w1 = rect1.width;
        let w2 = rect2.width;
        let newW: Fraction | number = 0, newH: Fraction | number = 0;

        if (this.isShortSlice()) {
            if (math.largerEq(h1, w2)) {
                const v = w2; w2 = h2; h2 = v;
            } else if (math.largerEq(h2, w1)) {
                const v = w1; w1 = h1; h1 = v;
            }
            newW = math.add(math.add(w1, w2), gap);
            newH = math.max(h1, h2);
        } else if (this.isLongSlice()) {
            newW = math.max(w1, w2);
            newH = math.add(math.add(h1, h2), gap);
        }

        this.width = math.max(newW, newH);
        this.height = math.min(newW, newH);
        this.isFraction = rect1.isFraction || rect2.isFraction;

        if (this.isFraction) {
            this.width = math.fraction(this.width);
            this.height = math.fraction(this.height);
        }

        this.minSize = math.min(rect1.minSize, rect2.minSize);
        this.gap = gap;
    }
}

const getRectNode = (name: string, widthStr: string = "0", heightStr: string = "0"): RectNode => {
    const f = isFraction(widthStr) || isFraction(heightStr);
    return new RectNode(name, parseSize(widthStr, f), parseSize(heightStr, f), 0.0, f);
};

export const getTree = (
    model: Model,
    solutionNumber: number,
    sheetNumber: number,
    gap: Fraction | number
): RectNode => {
    // Lookup tables: label -> Part, set of valid part labels, sheet# -> postfix tree string
    const partsMap = new Map<string, Part>();
    const idSet = new Set<string>();
    const sheetMap = new Map<number, string>();

    model.parts.forEach((part: Part) => {
        partsMap.set(part.label, part);
    });

    model.parts?.forEach((part: Part) => {
        idSet.add(part.label);
    });

    model.solutions?.[solutionNumber].sheets.forEach((sheet: SheetData) => {
        sheetMap.set(sheet.number, sheet.tree);
    });

    // Parse the postfix (RPN) expression: leaves become part/faux nodes, others become operations.
    const postfixArray = sheetMap.get(sheetNumber)!
        .split(' ')
        .map((label: string) => {
            if (idSet.has(label)) {
                const part = partsMap.get(label)!;
                return getRectNode(label, part.width, part.length); // real part
            } else {
                return getRectNode(label); // operation node (L/S)
            }
        });

    // Evaluate the postfix expression with a stack -> builds the binary tree.
    const stack: RectNode[] = [];
    postfixArray.forEach((element: RectNode) => {
        if (element.isOperation) {
            const right = stack.pop();
            const left = stack.pop();
            if (!right || !left) {
                throw new Error('Invalid layout.');
            }
            element.joinRects(left, right, gap); // gap is baked into the parent's size here
        }
        stack.push(element);
    });

    const enclosingRect = stack.pop();
    if (!enclosingRect) {
        throw new Error('Invalid layout.');
    }

    // If the sheet is bigger than the packed parts, add a faux (waste) rect to fill the sheet.
    if (model.sheets) {
        const sheetWidth = parseSize(model.sheets.width, enclosingRect.isFraction);
        const sheetHeight = parseSize(model.sheets.length, enclosingRect.isFraction);

        if (math.larger(sheetHeight, enclosingRect.height)) {
            const fauxRect = new RectNode("-", sheetWidth, math.subtract(sheetHeight, enclosingRect.height), 0.0);
            const sheetRect = getRectNode("L");
            sheetRect.joinRects(enclosingRect, fauxRect, 0.0);
            return sheetRect;
        } else if (math.larger(sheetWidth, enclosingRect.width)) {
            const fauxRect = new RectNode("-", math.subtract(sheetWidth, enclosingRect.width), sheetHeight, 0.0);
            const sheetRect = getRectNode("S");
            sheetRect.joinRects(enclosingRect, fauxRect, 0.0);
            return sheetRect;
        }
    }

    return enclosingRect;
};