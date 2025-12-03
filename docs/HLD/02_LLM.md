## VR.2 - LLM Integration Architecture

Multi-provider LLM integration with role-based model selection and customizable prompt engineering.

**Provider Support:**
- OpenAI (GPT models)
- Anthropic (Claude models)
- Google (Gemini models)
- Ollama (local models)
- OpenRouter (aggregated access)
- DeepSeek (specialized models)
- Custom (url, model, creds, with openai/ollama/etc. protocols);

**Dynamic Model Selection:**
- Role-based default model assignment
- Interactive model switching per project/task
- Performance and cost optimization based on task complexity
- Configured Default models for given roles (both global and per project);
- Ability to override model selection and parameters (thinking tokens, temperature etc.) for given task or even a single prompt;

Non-functional requirements:

* handle retries if request to model fails;
* handle rate limited APIs:
  * wait out for specified time;
  * switch to backup model provider(s);
* use information about input/output/cached tokens and and attach it to response along with estimated cost;
* context must be pruned if switching to a model with shorter context window;

### Interfaces

* LLM client interface with following functionalities:
  * streaming chat;
  * listing available models;
