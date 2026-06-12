import os
import subprocess

import yaml
from jinja2 import Environment, FileSystemLoader

# Configuration
TEMPLATE_DIR = "templates"
SOPS_FILE = "vars.yml"
OUTPUT_BASE = "output"
ISO_NAME = "ignition.iso"

FILES = {
    "config.ign.template": "ignition/config.ign",
    "script.template": "combustion/script",
}


def build():
    if not os.path.exists(SOPS_FILE):
        print(f"Error: {SOPS_FILE} not found.")
        return

    try:
        result = subprocess.run(
            ["sops", "decrypt", SOPS_FILE], capture_output=True, text=True, check=True
        )
    except FileNotFoundError:
        print("Error: 'sops' not found. Install it first.")
        return
    except subprocess.CalledProcessError as e:
        print(f"SOPS decryption failed:\n{e.stderr}")
        return

    try:
        config_vars = yaml.safe_load(result.stdout)
    except Exception as e:
        print(f"Error parsing decrypted YAML: {e}")
        return

    env = Environment(loader=FileSystemLoader(TEMPLATE_DIR))

    print("\nGenerating configuration files...")
    for tmpl_name, rel_path in FILES.items():
        full_path = os.path.join(OUTPUT_BASE, rel_path)
        os.makedirs(os.path.dirname(full_path), exist_ok=True)

        template = env.get_template(tmpl_name)
        rendered_content = template.render(config_vars)

        with open(full_path, "w") as f:
            f.write(rendered_content)
        print(f"  - Created: {full_path}")

    create_iso()


def create_iso():
    print(f"\nBuilding {ISO_NAME}...")

    cmd = [
        "mkisofs",
        "-full-iso9660-filenames",
        "-o",
        ISO_NAME,
        "-V",
        "ignition",
        "./output",
    ]

    try:
        subprocess.run(cmd, check=True)
        print(f"Successfully created {ISO_NAME}")
    except subprocess.CalledProcessError as e:
        print(f"Error creating ISO: {e}")
    except FileNotFoundError:
        print(
            "Error: 'mkisofs' command not found. Please install genisoimage or xorriso."
        )


if __name__ == "__main__":
    build()
