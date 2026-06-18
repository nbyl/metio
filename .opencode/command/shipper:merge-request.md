---
description: Build a merge request for the current codebase.
agent: build
---

Prepare a merge request from the current state of the project. Follow this workflow:

* Compile all changed binaries.
* Run local tests and linters.
* If all verification is good, create a git commit.
* Push the branch to github. Create a merge request or pull request if none exists, otherwise update the existing one. Set the flag to delete the source branch on merge.
* Update the github ticket to "in Review".
