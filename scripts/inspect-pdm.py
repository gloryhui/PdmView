import argparse
from collections import Counter
import json
from pathlib import Path
import re
from xml.parsers import expat


OBJECT_TYPES = frozenset({
    "RootObject", "Model", "Package", "Table", "Column", "Key", "Index",
    "IndexColumn", "Reference", "ReferenceJoin", "PhysicalDiagram",
    "TableSymbol", "ReferenceSymbol", "RectangleSymbol", "Shortcut",
    "Group", "TargetModel",
})


def inventory(path):
    parser = expat.ParserCreate(namespace_separator="}")
    definitions = {}
    references = Counter()
    objects = Counter()
    metadata = {}
    namespaces = set()
    element_count = 0
    duplicate_ids = 0
    unknown_objects = 0

    def declaration(version, encoding, _standalone):
        metadata["xml_version"] = version
        metadata["encoding"] = encoding or "UTF-8"

    def instruction(target, text):
        if target != "PowerDesigner":
            return
        match = re.search(r"""\bversion\s*=\s*(['"])(\d+(?:\.\d+)+)\1""", text)
        if match:
            metadata["powerdesigner_version"] = match[2]
        metadata["pdm_signature"] = bool(re.search(
            r"""\bsignature\s*=\s*(['"])PDM_DATA_MODEL_XML\1""", text
        ))

    def element(tag, attributes):
        nonlocal element_count, duplicate_ids, unknown_objects
        if element_count == 0:
            metadata["model_root"] = tag == "Model"
        element_count += 1
        namespace, _, kind = tag.rpartition("}")
        if namespace:
            namespaces.add(namespace)
        if namespace != "object":
            return
        if ref := attributes.get("Ref"):
            references[kind, ref] += 1
        if object_id := attributes.get("Id"):
            if object_id in definitions:
                duplicate_ids += 1
            else:
                definitions[object_id] = kind
            if kind in OBJECT_TYPES:
                objects[kind] += 1
            else:
                unknown_objects += 1

    def reject_doctype(*_):
        raise ValueError("XML document types are unsupported")

    parser.XmlDeclHandler = declaration
    parser.ProcessingInstructionHandler = instruction
    parser.StartElementHandler = element
    parser.StartDoctypeDeclHandler = reject_doctype
    with path.open("rb") as stream:
        parser.ParseFile(stream)
        size = stream.tell()
    return {
        **metadata,
        "bytes": size,
        "pdm_namespaces": sorted(namespaces & {"attribute", "collection", "object"}),
        "other_namespaces": len(namespaces - {"attribute", "collection", "object"}),
        "elements": element_count,
        "definitions": sum(objects.values()) + unknown_objects,
        "duplicate_ids": duplicate_ids,
        "references": sum(references.values()),
        "unresolved_references": sum(count for (_, ref), count in references.items() if ref not in definitions),
        "wrong_type_references": sum(count for (kind, ref), count in references.items()
                                     if ref in definitions and definitions[ref] != kind),
        "objects": dict(sorted(objects.items())),
        "other_object_definitions": unknown_objects,
    }


def main():
    arguments = argparse.ArgumentParser(description="Report aggregate XML structure without model text or IDs.")
    arguments.add_argument("file", type=Path)
    options = arguments.parse_args()
    try:
        result = inventory(options.file)
    except expat.ExpatError as error:
        arguments.exit(1, f"XML parsing failed at line {error.lineno}, column {error.offset}\n")
    except (OSError, ValueError):
        arguments.exit(1, "Could not read supported XML input\n")
    print(json.dumps(result, indent=2, ensure_ascii=True))


if __name__ == "__main__":
    main()
