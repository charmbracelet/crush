def greet(name: str) -> str:
    """Return a greeting message."""
    return f"Hello, {name}!"


def add(a: int, b: int) -> int:
    """Add two integers."""
    return a + b


class Calculator:
    def __init__(self, value: int = 0):
        self.value = value

    def multiply(self, x: int) -> int:
        self.value *= x
        return self.value
