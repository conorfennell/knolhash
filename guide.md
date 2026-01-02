# A Guide to Creating Great Knolhash Entries

This guide outlines the principles and best practices for creating high-quality, effective Q/A/C (Question/Answer/Context) entries for the `knolhash` spaced repetition system. The goal is to create atomic, precise, and durable knowledge that is easy to review and remember.

## Core Principles

1.  **Atomicity: One Idea Per Card**
    This is the most important principle. Each card should represent a single, irreducible piece of information. Avoid lumping multiple facts into one card.

    *   **Bad:** "What is Go and who created it?"
    *   **Good:**
        *   Card 1: "What is Go?"
        *   Card 2: "Who created the Go programming language?"

2.  **Clarity and Precision**
    The question should be unambiguous and lead to a single, correct answer. If you have to think, "What is this question *really* asking?" it's a bad question.

3.  **Personalization**
    Frame questions in a way that makes sense to *you*. Use your own words and mental models. The system is for you, so the knowledge should be in your "language."

## The Q: Field (The Question)

The question is the prompt that triggers recall.

*   **Be Specific:** Avoid vague questions.
    *   **Bad:** `Q: Go concurrency?`
    *   **Good:** `Q: What is the primary mechanism for managing concurrency in Go?`

*   **Use "Wh-" Questions:** *Who, What, Where, When, Why, How* are excellent starting points for questions.
    *   `Q: Why was the `go` keyword introduced in Go?`

*   **Use Fill-in-the-Blank Prompts:** Create a sentence with a key term missing.
    *   `Q: In Go, a ________ is used to send and receive values between goroutines.`
    *   `A: channel`

## The A: Field (The Answer)

The answer should be concise and directly address the question.

*   **Be Concise:** Answer only what the question asks. No extra information.
*   **Format for Readability:** Use Markdown to structure the answer clearly. For code, always use code blocks.

    ```
    Q: How do you start a web server in Go on port 8080?
    A:
    ```go
    http.ListenAndServe(":8080", nil)
    ```
    ```

## The C: Field (The Context)

The context acts as a tag or category. It helps group related cards and provides a mental anchor for the information.

*   **Use a Hierarchy:** Use slashes (`/`) or dots (`.`) to create a topic hierarchy. This helps organize your knowledge base.
    *   `C: Programming/Go/Concurrency`
    *   `C: History.WW2.EasternFront`

*   **Be Consistent:** Use the same context for related cards. This will be powerful for future features like studying by topic.

*   **Use Multiple Contexts:** If a card belongs to multiple topics, you can use commas.
    *   `C: Go, Web, HTMX`

---

## Examples

### Bad Example (Too Broad)

```
Q: Tell me about Go.
A: Go is a statically typed, compiled programming language designed at Google by Robert Griesemer, Rob Pike, and Ken Thompson. It is syntactically similar to C, but with memory safety, garbage collection, structural typing, and CSP-style concurrency.
C: Programming
```
**Critique:** This card tests at least five different facts. It violates the principle of atomicity.

### Good Examples (Broken Down)

```
Q: What company originally designed the Go programming language?
A: Google
C: Programming/Go/History

---

Q: Is Go a statically or dynamically typed language?
A: Statically typed
C: Programming/Go/Typing

---

Q: What feature of Go automatically manages memory allocation and de-allocation?
A: Garbage collection
C: Programming/Go/Memory

---

Q: Go's concurrency model is described as ______-style concurrency.
A: CSP (Communicating Sequential Processes)
C: Programming/Go/Concurrency
```

---

## Template for LLM Prompting

You can use the following template to instruct a Large Language Model (LLM) to generate cards for you.

```text
You are an expert in knowledge formulation for spaced repetition systems. Your task is to transform the following text into a series of atomic Q/A/C entries for my `knolhash` system.

Adhere strictly to these principles:
1.  **Atomicity:** One single, irreducible idea per card.
2.  **Clarity:** The question must be unambiguous.
3.  **Formatting:**
    *   Use Markdown for answers (especially code blocks for code).
    *   Use a hierarchical format for the Context field (e.g., `Topic/Sub-Topic`).

Here is an example of the desired output format:
---
Q: What is the primary mechanism for managing concurrency in Go?
A: Goroutines and channels
C: Programming/Go/Concurrency
---
Q: What Go keyword is used to start a new goroutine?
A: `go`
C: Programming/Go/Concurrency
---

Now, please process the following text and generate a series of Q/A/C entries separated by `---`:

[PASTE YOUR TEXT HERE]
```
