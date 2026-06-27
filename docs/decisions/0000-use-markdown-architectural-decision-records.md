---
parent: Decisions
nav_order: 0
status: accepted
date: 12026-06-23
decision-makers: Yunseo Kim
---

# Use Markdown Architectural Decision Records

## Context and Problem Statement

We want to record architectural decisions made in this project independent whether decisions concern the architecture ("architectural decision record"), the code, or other fields.
Which format and structure should these records follow?

## Decision Drivers

- Architectural decisions should be explicit and discoverable later.
- Decision records should be structured enough to compare alternatives.
- The format should stay lightweight enough for day-to-day development.
- The convention should apply to architecture, code, documentation, and operating-policy decisions.
- The chosen template should be maintained and recognizable outside this project.

## Considered Options

- [MADR](https://adr.github.io/madr/) 4.0.0 – The Markdown Architectural Decision Records
- [Michael Nygard's template](http://thinkrelevance.com/blog/2011/11/15/documenting-architecture-decisions) – The first incarnation of the term "ADR"
- [Sustainable Architectural Decisions](https://www.infoq.com/articles/sustainable-architectural-design-decisions) – The Y-Statements
- Other templates listed at <https://github.com/joelparkerhenderson/architecture_decision_record>
- Formless – No conventions for file format and structure

## Decision Outcome

Chosen option: "MADR 4.0.0", because it provides a lean, structured, and maintained Markdown format for capturing decisions across architecture, code, documentation, and operating policy.

### Consequences

- Good, because implicit assumptions are made explicit for future maintainers.
- Good, because a shared structure makes decisions easier to compare, review, and update.
- Good, because MADR is lightweight enough to fit this repository's documentation-first workflow.
- Bad, because contributors need to follow a template rather than writing fully free-form notes.

### Confirmation

Compliance with this decision is confirmed when new decision records are added under `docs/decisions/` using the MADR structure recorded in this ADR.
Decision records should include context, decision drivers, considered options, outcome, consequences, confirmation, option tradeoffs, and more information when relevant.

## Pros and Cons of the Options

### MADR 4.0.0

The Markdown Architectural Decision Records template provides a structured Markdown format for documenting decisions.

- Good, because implicit assumptions should be made explicit.
  Design documentation is important to enable people understanding the decisions later on.
  See also ["A rational design process: How and why to fake it"](https://doi.org/10.1109/TSE.1986.6312940).
- Good, because MADR allows for structured capturing of any decision.
- Good, because the MADR format is lean and fits our development style.
- Good, because the MADR structure is comprehensible and facilitates usage and maintenance.
- Good, because the MADR project is vivid.
- Neutral, because it introduces a formal convention that contributors must learn.

### Michael Nygard's template

The first incarnation of the term "ADR" and a widely recognized lightweight ADR structure.

- Good, because it is simple and historically influential.
- Good, because many engineers recognize it.
- Bad, because it is less explicit about decision drivers, option tradeoffs, and confirmation.

### Sustainable Architectural Decisions

The Y-Statement style emphasizes concise decision statements and sustainability of architecture documentation.

- Good, because it encourages concise framing.
- Good, because it focuses on sustainable architecture decision-making.
- Bad, because it is less directly aligned with the full Markdown document structure we want for this repository.

### Other ADR templates

Other templates are available in community collections.

- Good, because there are many specialized alternatives.
- Neutral, because template choice can be revisited if MADR stops fitting the project.
- Bad, because choosing an ad hoc alternative would require additional local convention work.

### Formless

No fixed decision-record convention.

- Good, because it imposes no upfront structure.
- Bad, because decisions would become harder to compare and maintain.
- Bad, because future contributors would need to infer expected structure from examples.
- Bad, because important assumptions and tradeoffs could be omitted accidentally.

## More Information

The full MADR template to use for future decisions is recorded below.

```markdown
---
# Configuration for the Jekyll template "Just the Docs"
parent: Decisions
nav_order: 100

# These are optional elements. Feel free to remove any of them.
# status: "{proposed | rejected | accepted | deprecated | … | superseded by ADR-0123"
# date: {YYYYY-MM-DD when the decision was last updated, using Holocene Era / Human Era year format}
# decision-makers: {list everyone involved in the decision}
# consulted: {list everyone whose opinions are sought (typically subject-matter experts); and with whom there is a two-way communication}
# informed: {list everyone who is kept up-to-date on progress; and with whom there is a one-way communication}
---

# {short title, representative of solved problem and found solution}

## Context and Problem Statement

{Describe the context and problem statement, e.g., in free form using two to three sentences or in the form of an illustrative story. You may want to articulate the problem in form of a question and add links to collaboration boards or issue management systems.}

<!-- This is an optional element. Feel free to remove. -->

## Decision Drivers

- {decision driver 1, e.g., a force, facing concern, …}
- {decision driver 2, e.g., a force, facing concern, …}
- … <!-- numbers of drivers can vary -->

## Considered Options

- {title of option 1}
- {title of option 2}
- {title of option 3}
- … <!-- numbers of options can vary -->

## Decision Outcome

Chosen option: "{title of option 1}", because {justification. e.g., only option, which meets k.o. criterion decision driver | which resolves force {force} | … | comes out best (see below)}.

<!-- This is an optional element. Feel free to remove. -->

### Consequences

- Good, because {positive consequence, e.g., improvement of one or more desired qualities, …}
- Bad, because {negative consequence, e.g., compromising one or more desired qualities, …}
- … <!-- numbers of consequences can vary -->

<!-- This is an optional element. Feel free to remove. -->

### Confirmation

{Describe how the implementation of/compliance with the ADR can/will be confirmed. Is the chosen design and its implementation in line with the decision? E.g., a design/code review or a test with a library such as ArchUnit can help validate this. Note that although we classify this element as optional, it is included in many ADRs.}

<!-- This is an optional element. Feel free to remove. -->

## Pros and Cons of the Options

### {title of option 1}

<!-- This is an optional element. Feel free to remove. -->

{example | description | pointer to more information | …}

- Good, because {argument a}
- Good, because {argument b}
<!-- use "neutral" if the given argument weights neither for good nor bad -->
- Neutral, because {argument c}
- Bad, because {argument d}
- … <!-- numbers of pros and cons can vary -->

### {title of other option}

{example | description | pointer to more information | …}

- Good, because {argument a}
- Good, because {argument b}
- Neutral, because {argument c}
- Bad, because {argument d}
- …

<!-- This is an optional element. Feel free to remove. -->

## More Information

{You might want to provide additional evidence/confidence for the decision outcome here and/or document the team agreement on the decision and/or define when/how this decision the decision should be realized and if/when it should be re-visited. Links to other decisions and resources might appear here as well.}
```
