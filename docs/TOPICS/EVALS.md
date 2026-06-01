# Question

Find materials, papers, sample projects regarding Agentic Evaluation specialized for Code Agents. I want to be able to measure performance of my code agent when swapping following components::
* various LLM (or LLMs when multiple are used in the same task)
* various context management strategies
* changes in system prompt and tools visibility

Here are my preferences for the evaluation:
* it should be repeatable and reproducible
* it should be fully automated (i.e. being able to automatically determine if results are correct, not requiring human to manually check every result)
* it should have big enough evaluation dataset
  * I can compile a dataset from many existing datasets if provided
* it should be fully open source, able to run locally (as it will be tested with local models too)
* papers and sample projects
* publications, blog posts and papers comprehensively summarizing existing evaluation approaches, including state of the art techniques
* publications, blog posts and papers describing how to create datasets for code agents
* publications, blog posts and papers describing how to evaluate code agents, what are the pitfalls etc.

I will implement my evaluation framework and run it on my code agent to measure performance, so none of projects you find has to be working end-to-end, I'm looking mainly for ideas and sample fragments of implementations and datasets (or techniquest to create datasets). My framework will be tailored to a particular code agent harness and will be used to evaluate its performance end to end (and to detect regressions when changing things like compaction algorithms, system prompt, model and model parameters etc.).

Please find as many relevant materials as possible, do not limit yourself to just a few sources.
