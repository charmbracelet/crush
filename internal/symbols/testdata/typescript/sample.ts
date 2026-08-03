export function greet(name: string): string {
  return `Hello, ${name}!`;
}

export function add(a: number, b: number): number {
  return a + b;
}

export class Calculator {
  value: number;

  constructor(value: number = 0) {
    this.value = value;
  }

  multiply(x: number): number {
    this.value *= x;
    return this.value;
  }
}
