Source: https://google.github.io/styleguide/go/\n\n# styleguide\n\n\n# styleguide\n\n# Go Style\n\nhttps://google.github.io/styleguide/go\n\nOverview | Guide | Decisions |
Best practices\n\n\n\n## About\n\nThe Go Style Guide and accompanying documents codify the current best approaches
for writing readable and idiomatic Go. Adherence to the Style Guide is not
intended to be absolute, and these documents will never be exhaustive. Our
intention is to minimize the guesswork of writing readable Go so that newcomers
to the language can avoid common mistakes. The Style Guide also serves to unify
the style guidance given by anyone reviewing Go code at Google.\n\n\n\n### Documents\n\n1. The Style Guide outlines
the foundation of Go style at Google. This document is definitive and is
used as the basis for the recommendations in Style Decisions and Best
Practices.\n2. Style Decisions is a
more verbose document that summarizes decisions on specific style points and
discusses the reasoning behind the decisions where appropriate.
These decisions may occasionally change based on new data, new language
features, new libraries, or emerging patterns, but it is not expected that
individual Go programmers at Google should keep up-to-date with this
document.\n3. Best Practices
documents some of the patterns that have evolved over time that solve common
problems, read well, and are robust to code maintenance needs.
These best practices are not canonical, but Go programmers at Google are
encouraged to use them where possible to keep the codebase uniform and
consistent.\n\nThese documents intend to:\n\n- Agree on a set of principles for weighing alternate styles\n- Codify settled matters of Go style\n- Document and provide canonical examples for Go idioms\n- Document the pros and cons of various style decisions\n- Help minimize surprises in Go readability reviews\n- Help readability mentors use consistent terminology and guidance\n\nThese documents do not intend to:\n\n- Be an exhaustive list of comments that can be given in a readability review\n- List all of the rules everyone is expected to remember and follow at all
times\n- Replace good judgment in the use of language features and style\n- Justify large-scale changes to get rid of style differences\n\nThere will always be differences from one Go programmer to another and from one
team’s codebase to another. However, it is in the best interest of Google and
Alphabet that our codebase be as consistent as possible. (See
guide for more on consistency.) To that end, feel free to
make style improvements as you see fit, but you do not need to nit-pick every
violation of the Style Guide that you find. In particular, these documents may
change over time, and that is no reason to cause extra churn in existing
codebases; it suffices to write new code using the latest best practices and
address nearby issues over time.\n\nIt is important to recognize that issues of style are inherently personal and
that there are always inherent trade-offs. Much of the guidance in these
documents is subjective, but just like with gofmt, there is significant value
in the uniformity they provide. As such, style recommendations will not be
changed without due discourse, Go programmers at Google are encouraged to follow
the style guide even where they might disagree.\n\n\n\n## Definitions\n\nThe following words, which are used throughout the style documents, are defined
below:\n\n- Canonical: Establishes prescriptive and enduring rules

Within these documents, “canonical” is used to describe something that is
considered a standard that all code (old and new) should follow and that is
not expected to change substantially over time. Principles in the canonical
documents should be understood by authors and reviewers alike, so everything
included within a canonical document must meet a high bar. As such,
canonical documents are generally shorter and prescribe fewer elements of
style than non-canonical documents.
https://google.github.io/styleguide/go#canonical\n- Normative: Intended to establish consistency 
Within these documents, “normative” is used to describe something that is an
agreed-upon element of style for use by Go code reviewers, in order that the
suggestions, terminology, and justifications are consistent. These elements
may change over time, and these documents will reflect such changes so that
reviewers can remain consistent and up-to-date. Authors of Go code are not
expected to be familiar with the normative documents, but the documents will
frequently be used as a reference by reviewers in readability reviews.
https://google.github.io/styleguide/go#normative\n- Idiomatic: Common and familiar 
Within these documents, “idiomatic” is used to refer to something that is
prevalent in Go code and has become a familiar pattern that is easy to
recognize. In general, an idiomatic pattern should be preferred to something
unidiomatic if both serve the same purpose in context, as this is what will
be the most familiar to readers.
https://google.github.io/styleguide/go#idiomatic\n\n\n\n## Additional references\n\nThis guide assumes the reader is familiar with Effective Go, as it provides a
common baseline for Go code across the entire Go community.\n\nBelow are some additional resources for those looking to self-educate about Go
style and for reviewers looking to provide further linkable context in their
reviews. Participants in the Go readability process are not expected to be
familiar with these resources, but they may arise as context in readability
reviews.\n\nExternal References\n\n- Go Language Specification\n- Go FAQ\n- Go Memory Model\n- Go Data Structures\n- Go Interfaces\n- Go Proverbs\n- Go Tip Episodes - stay tuned.\n- Unit Testing Practices - stay tuned.\n\nRelevant Testing-on-the-Toilet articles\n\n- TotT: Identifier Naming\n- TotT: Testing State vs. Testing Interactions\n- TotT: Effective Testing\n- TotT: Risk-driven Testing\n- TotT: Change-detector Tests Considered Harmful\n\nAdditional External Writings\n\n- Go and Dogma\n- Less is exponentially more\n- Esmerelda’s Imagination\n- Regular expressions for parsing\n- Gofmt’s style is no one’s favorite, yet Gofmt is everyone’s favorite
(YouTube)