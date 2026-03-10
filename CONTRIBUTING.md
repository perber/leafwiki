# Contributing to LeafWiki

First of all: **thank you for your interest in contributing to LeafWiki!**

LeafWiki is built to be a **simple, reliable and low-overhead wiki for structured documentation**.  
Contributions that improve the project while keeping these goals intact are always welcome.

Whether you're fixing a typo, reporting a bug, improving documentation, or contributing code — every contribution helps.

---

# Ways to contribute

There are many ways to help improve LeafWiki:

- Reporting bugs
- Suggesting improvements or ideas
- Improving documentation
- Fixing bugs
- Implementing new features

If you're unsure whether something is worth contributing — **open an issue and ask**.

---

# Project principles

LeafWiki intentionally follows a small set of design principles.

Contributions should respect these ideas:

- **Markdown stored on disk**
- **Minimal operational complexity**
- **Single binary deployment**
- **Explicit structure over hidden automation**
- **Self-hosting friendly**

---

# Before submitting code

For **anything larger than a small fix**, please open an issue first.

This helps to:

- avoid duplicate work
- ensure the change fits the project goals
- discuss possible approaches

Small improvements, documentation updates, and obvious bug fixes can usually be submitted directly.

---

# Development setup

Clone the repository:

``` bash
git clone https://github.com/perber/leafwiki.git
cd leafwiki
```

Start the frontend:

``` bash
cd ui/leafwiki-ui
npm install
npm run dev
```

Start the backend (in another terminal):

``` bash
cd cmd/leafwiki
go run main.go --jwt-secret=dev --admin-password=dev --public-access=true --allow-insecure=true
```

The frontend dev server usually runs at:

http://localhost:5173

---

Before submitting the code execute `npm run format` in the `ui/leafwiki-ui` directory.
If you change the e2e tests, please also run npm run format in the `e2e` directory.

# Pull request guidelines

To keep reviews efficient, please follow these guidelines:

- Keep pull requests **small and focused**
- Clearly **describe the change**
- Reference related **issues**
- Avoid unrelated formatting changes
- Update documentation if necessary

PRs that combine multiple unrelated changes may be asked to split.

---

# Commit style

LeafWiki uses **Conventional Commits**.

Please format commit messages like this:

```
type(scope): short description
```

Examples:

```
feat(editor): add keyboard shortcut for headings
fix(search): handle empty query correctly
docs(readme): clarify installation instructions
refactor(api): simplify page loading logic
```

Common commit types:

- `feat` – new functionality
- `fix` – bug fix
- `docs` – documentation changes
- `refactor` – internal improvements
- `chore` – maintenance work
- `test` – tests

---

# Feature proposals

If you'd like to propose a feature, please include:

- the **problem or use case**
- why existing functionality is insufficient
- a rough idea of the expected behavior

Good feature requests focus on **practical real-world usage**.

---

# Documentation contributions

Documentation improvements are always welcome.

Examples include:

- improving explanations
- fixing errors or typos
- adding examples
- clarifying installation steps

---

# Code style

Please follow the existing project style.

General guidelines:

- prefer clarity over cleverness
- avoid introducing unnecessary abstractions
- keep the code easy to understand and maintain

---

# Review process

All contributions are reviewed before merging.

During review we may:

- ask for small improvements
- suggest alternative implementations
- discuss design decisions

This helps keep the project consistent and maintainable.

---

# Questions or discussions

If you're unsure about something, feel free to open an issue or discussion.

Feedback and ideas are always welcome.

---

Thanks again for helping improve **LeafWiki** 🌿
