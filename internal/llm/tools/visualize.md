# Visualize Tool

The `visualize` tool generates ASCII diagrams to help developers understand complex code structures and relationships at a glance. It supports various visualization types, including class diagrams, function call graphs, and data flow diagrams.

## Available Visualization Types

- **`class`**: Generates UML-like class diagrams showing classes, methods, and relationships.
- **`function`**: Generates function call graphs showing how functions call each other.
- **`dataflow`**: Generates data flow diagrams showing how data moves through the system.

## Parameters

The `visualize` tool accepts the following parameters:

- **`file_path`** (string, required):
  The absolute path to the file you want to visualize.

- **`type`** (string, required):
  The type of visualization to generate. Must be one of `class`, `function`, or `dataflow`.

- **`title`** (string, optional):
  An optional title for the generated diagram.

- **`description`** (string, optional):
  An optional description for the generated diagram.

## Examples

### Class Diagram

```
+-----------------------------------------------------+
|                    Class Diagram                    |
+-----------------------------------------------------+

+---------------------+        +---------------------+
|     ClassName       |        |    AnotherClass     |
+---------------------+        +---------------------+
| - privateField: Type|        | - field: Type       |
| + publicMethod()    |<-------| + method(): ReturnType
+---------------------+        +---------------------+
         ^                              ^
         |                              |
         | Inherits                     | Uses
         |                              |
+---------------------+        +---------------------+
|   SubClassName      |        |    HelperClass      |
+---------------------+        +---------------------+
| + methodOverride()  |        | + utilityMethod()   |
+---------------------+        +---------------------+

Legend:
  + : Public
  - : Private
  <------- : Association
  ^ : Inheritance
```

### Function Call Graph

```
+-----------------------------------------------------+
|                 Function Call Graph                 |
+-----------------------------------------------------+

         +-----------+
         | main()    |
         +-----------+
              |
      +-------+-------+
      |               |
      v               v
+-----------+   +-----------+
| init()    |   | process() |
+-----------+   +-----------+
                    |
            +-------+-------+
            |               |
            v               v
      +-----------+   +-----------+
      | validate()|   | save()    |
      +-----------+   +-----------+
                          |
                          v
                    +-----------+
                    | notify()  |
                    +-----------+

Legend:
  Rectangles: Functions
  Arrows: Function calls
```

### Data Flow Diagram

```
+-----------------------------------------------------+
|                  Data Flow Diagram                  |
+-----------------------------------------------------+

  User Input        Database         External API
      |                 |                 |
      v                 |                 |
+-----------+           |                 |
|  Form     |           |                 |
| Validation|           |                 |
+-----------+           |                 |
      |                 |                 |
      +--------->-------+                 |
                        |                 |
                        v                 |
                  +-----------+           |
                  |  Process  |           |
                  |   Data    |           |
                  +-----------+           |
                        |                 |
                        +--------->-------+
                                          |
                                          v
                                    +-----------+
                                    |   Send    |
                                    |  Results  |
                                    +-----------+
                                          |
                                          v
                                   +-----------+
                                   |  Display  |
                                   | Results   |
                                   +-----------+

Legend:
  Rectangles: Data processors
  Arrows: Data flow direction
```
