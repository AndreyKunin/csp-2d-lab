// A minimal DOM, enough for react-dom to mount and update simple components in
// Node (no DOM library is a dependency). Importing it installs the globals;
// node --test runs each test file in its own process, so only the files that
// import it see them. Events are not dispatched.

class FakeNode {
    childNodes: FakeNode[] = [];
    parentNode: FakeNode | null = null;
    attributes: { [k: string]: string } = {};
    style: any = {setProperty() {}, removeProperty() {}};

    constructor(public nodeType: number, public nodeName: string, public ownerDocument: any, public data = '') {}

    get firstChild(): FakeNode | null { return this.childNodes[0] ?? null; }
    get lastChild(): FakeNode | null { return this.childNodes[this.childNodes.length - 1] ?? null; }
    get nextSibling(): FakeNode | null {
        const siblings = this.parentNode?.childNodes;
        return siblings ? siblings[siblings.indexOf(this) + 1] ?? null : null;
    }
    get tagName(): string { return this.nodeName; }
    get namespaceURI(): string { return 'http://www.w3.org/1999/xhtml'; }
    get nodeValue(): string { return this.data; }
    set nodeValue(v: string) { this.data = v; }
    get textContent(): string {
        return this.nodeType === 3 ? this.data : this.childNodes.map(c => c.textContent).join('');
    }
    set textContent(v: string) {
        this.childNodes = [];
        if (v) {
            this.appendChild(new FakeNode(3, '#text', this.ownerDocument, v));
        }
    }

    appendChild(child: FakeNode): FakeNode {
        child.parentNode?.removeChild(child);
        this.childNodes.push(child);
        child.parentNode = this;
        return child;
    }
    insertBefore(child: FakeNode, ref: FakeNode | null): FakeNode {
        if (!ref) {
            return this.appendChild(child);
        }
        child.parentNode?.removeChild(child);
        this.childNodes.splice(this.childNodes.indexOf(ref), 0, child);
        child.parentNode = this;
        return child;
    }
    removeChild(child: FakeNode): FakeNode {
        this.childNodes.splice(this.childNodes.indexOf(child), 1);
        child.parentNode = null;
        return child;
    }
    setAttribute(k: string, v: string) { this.attributes[k] = String(v); }
    removeAttribute(k: string) { delete this.attributes[k]; }
    getAttribute(k: string) { return this.attributes[k] ?? null; }
    hasAttribute(k: string) { return k in this.attributes; }
    addEventListener() {}
    removeEventListener() {}
}

export const fakeDocument: any = {
    createElement: (tag: string) => new FakeNode(1, tag.toUpperCase(), fakeDocument),
    createElementNS: (_ns: string, tag: string) => new FakeNode(1, tag.toUpperCase(), fakeDocument),
    createTextNode: (text: string) => new FakeNode(3, '#text', fakeDocument, text),
    addEventListener() {},
    removeEventListener() {},
};
fakeDocument.documentElement = fakeDocument.createElement('html');
fakeDocument.body = fakeDocument.createElement('body');

(globalThis as any).document = fakeDocument;
(globalThis as any).window = {
    document: fakeDocument, addEventListener() {}, removeEventListener() {},
    setInterval: () => 1, clearInterval() {}, event: undefined,
    HTMLIFrameElement: class {}, HTMLElement: FakeNode, Node: FakeNode,
};
(globalThis as any).IS_REACT_ACT_ENVIRONMENT = true;

export type {FakeNode};
