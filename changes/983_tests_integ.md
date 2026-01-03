Rework integration tests for openai, anthropic and ollama to use files in `_integ` directory (at project root) to
provide configuration and enabling switch:

* always run all integration tests if file `_integ/all.enabled` exists and contains `yes` string (case insensitive, trim spaces);
* run just ollama tests if file `_integ/ollama.enabled` exists and contains `yes` string (case insensitive, trim spaces);
* run just openai tests if file `_integ/openai.enabled` exists and contains `yes` string (case insensitive, trim spaces);
* run just anthropic only if file `_integ/anthropic.enabled` exists and contains `yes` string (case insensitive, trim spaces);
* fetch URL to ollama from file `_integ/ollama.url` instead of environment variable;
* fetch URL to openai from file `_integ/openai.url` instead of environment variable;
* fetch optional API key for openai from file `_integ/openai.key`, do not use API key if file does not exist;
* fetch optional API key for ollama from file `_integ/ollama.key`, do not use API key if file does not exist;
* fetch API key for anthropic from file `_integ/anthropic.key`;
* tests for mock models should not be affected and always run;
* all above files can contain extra spaces, be sure to trim spaces after reading content;
* be sure to not modify tests behavior in any other way;
