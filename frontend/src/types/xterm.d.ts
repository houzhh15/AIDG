declare module 'xterm' {
  export class Terminal {
    constructor(options?: any);
    open(container: HTMLElement): void;
    write(data: string): void;
    writeln(data: string): void;
    onData(cb: (data: string) => void): void;
    dispose(): void;
    rows: number;
    cols: number;
  }
}