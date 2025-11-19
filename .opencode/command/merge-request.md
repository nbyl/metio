---
description: Build a merge request for the current codebase.
agent: build
---

Prepare a merge request from the current state of the project. Follow this workflow:

* Compile all changed binaries. 
* Run local tests.
* If all tests are good, create a git commit.
* Push the branch to GitLab. Create a merge request if none exists, otherwise update the existing one. Set the flag to delete the source branch on merge.
* Update the linear ticket to "in Review".