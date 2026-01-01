# Comprehensive Test Cases for Knolhash Parser

## Standard Cards
Q: What is the capital of Ireland?
A: Dublin

Q: What is the primary function of a CPU?
A: To execute instructions from a computer program.
C: Computer Architecture

## Multi-line Fields

Q: List the first five books of the Old Testament.
A: 
Genesis
Exodus
Leviticus
Numbers
Deuteronomy
C: Religion

Q: What are the key principles of SOLID?
A:
- **S**ingle Responsibility Principle
- **O**pen/Closed Principle
- **L**iskov Substitution Principle
- **I**nterface Segregation Principle
- **D**ependency Inversion Principle
C: Software Design

## Code Blocks

Q: How do you declare a variable in Go?
A: Using the `var` keyword or the `:=` short declaration statement.
`var name string = "Conor"`
`age := 42`
C: Go Programming

Q: Write a simple "Hello, World" program in Python.
A:
```python
def main():
    print("Hello, World!")

if __name__ == "__main__":
    main()
```

## Special Characters & Formatting

Q: Is this a valid question? " ' / \ `?
A: Yes, it is. All special characters should be handled.

Q: Let's test markdown: *italic*, **bold**, and `code`.
A: The parser should preserve the raw text content, not interpret the markdown.

## Irregular Spacing and Ordering

Q:    This question has leading whitespace.
A: And this answer has some.    
C:      And so does this context.


Q: This card is separated by many blank lines.
A: The parser should handle it.



Q: This card has its fields out of order.
C: The current parser might struggle with this.
A: This is the answer.

## Edge Cases

Q:This question has no space after the prefix.
A:This answer also has no space.

Q: What happens with a question but no answer?

Q: This is the last card in the file with no trailing newline.
A: It should be parsed correctly.