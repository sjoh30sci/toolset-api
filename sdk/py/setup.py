from setuptools import setup, find_packages

with open("README.md", "r", encoding="utf-8") as fh:
    long_description = fh.read()

setup(
    name="toolset-api",
    version="0.1.0",
    description="Python client for the Toolset API (search, files, exec, browser, MCP)",
    long_description=long_description,
    long_description_content_type="text/markdown",
    author="yourusername",
    url="https://github.com/yourusername/toolset-api",
    packages=find_packages(include=["toolset_api", "toolset_api.*"]),
    install_requires=["requests>=2.28.0"],
    python_requires=">=3.8",
    license="MIT",
    classifiers=[
        "Programming Language :: Python :: 3",
        "License :: OSI Approved :: MIT License",
        "Operating System :: OS Independent",
    ],
)
