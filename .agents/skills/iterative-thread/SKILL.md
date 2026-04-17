---
name: iterative-thread
description: Doing complex task by iterating them piece by piece
---

This skill applies to tasks that are too complex to fit into context window but can be done in smaller steps linked
by code dependencies. Each task consists of initial step and many follow-up steps.

How to work with such task:

* perform initial step, while avoiding to do thorough analysis of the whole task (i.e. change or remoe some definition in single file)
* iteratively perform follow-up steps one by one by doing following iteration:
  * run full build and all tests to expose first file that is affected by the change and needs to be changed or fixed
  * perform fix or change only on this one file
  * run full build and all tests to check if the change is fixed and to expose next file that is affected by the change, etc.

DO NOT try to fix all files at once, proceed one by one in iterations as described above.

At the end run full build and all tests to confirm all works.
