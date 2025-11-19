---
description: Implement the planned changes using a fully featured workflow.
agent: build
---

Implement the currently discussed and planned changes. Follow this workflows:

* Change to the main branch and pull the latest changes from origin into it.
* Create a new feature branch based on the discussed ticket in linear. Use the linear tool.
* Make proposed changes to the code base.
* Compile all changed binaries. 
* Run local tests.
* If all tests are good, create a git commit.
* Push the branch to GitLab. Create a merge request if none exists, otherwise create one.
* Update the linear ticket to "in Review".