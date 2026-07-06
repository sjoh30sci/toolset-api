# toolset-api (Python SDK)

Python client for the [Toolset API](https://github.com/yourusername/toolset-api).

## Install

```bash
pip install toolset-api
```

## Usage

```python
from toolset_api import ToolsetAPI

client = ToolsetAPI(base_url="http://localhost:8080")

# Search
results = client.search(query="golang")

# Execute code
output = client.exec(code='print("Hello")', language="python")
```
