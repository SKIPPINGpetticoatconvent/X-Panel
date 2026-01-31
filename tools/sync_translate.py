import os
import re

TRANS_DIR = "/home/ub/X-Panel/web/translation"
MASTER_FILE = os.path.join(TRANS_DIR, "translate.en_US.toml")
EXCLUDE_FILES = ["translate.en_US.toml", "translate.zh_CN.toml", "translate.zh_TW.toml"]


def parse_tgbot_messages(content):
    in_section = False
    messages = {}
    lines = content.split("\n")
    for line in lines:
        stripped = line.strip()
        if stripped == "[tgbot.messages]":
            in_section = True
            continue
        if (
            in_section
            and stripped.startswith("[")
            and not stripped.startswith("[tgbot.messages]")
        ):
            break
        if in_section and "=" in line and not line.startswith("#"):
            parts = line.split("=", 1)
            key = parts[0].strip().strip('"').strip("'")
            val = parts[1].strip()
            messages[key] = val
    return messages


def to_html(text):
    # **text** -> <b>text</b>
    # Use non-greedy match
    text = re.sub(r"\*\*(.*?)\*\*", r"<b>\1</b>", text)
    # `text` -> <code>text</code>
    text = re.sub(r"`(.*?)`", r"<code>\1</code>", text)
    return text


def process_file(filepath, master_keys):
    with open(filepath, "r", encoding="utf-8") as f:
        content = f.read()

    lines = content.split("\n")
    new_lines = []
    in_messages = False
    existing_keys = set()

    for line in lines:
        stripped = line.strip()

        if stripped == "[tgbot.messages]":
            in_messages = True
            new_lines.append(line)
            continue

        if (
            in_messages
            and stripped.startswith("[")
            and not stripped.startswith("[tgbot.messages]")
        ):
            # End of section, insert missing keys here
            # Add a blank line before inserting
            new_lines.append("")
            for k, v in master_keys.items():
                if k not in existing_keys:
                    new_lines.append(f"{k} = {v}")
            in_messages = False
            new_lines.append(line)
            continue

        if in_messages and "=" in line and not line.startswith("#"):
            key_part = line.split("=")[0].strip().strip('"').strip("'")
            if key_part in existing_keys:
                continue  # Skip duplicate key
            existing_keys.add(key_part)
            # Apply transformations
            new_line = to_html(line)
            new_lines.append(new_line)
        else:
            new_lines.append(line)

    # If file ends inside [tgbot.messages]
    if in_messages:
        new_lines.append("")
        for k, v in master_keys.items():
            if k not in existing_keys:
                new_lines.append(f"{k} = {v}")

    with open(filepath, "w", encoding="utf-8") as f:
        f.write("\n".join(new_lines))


def main():
    if not os.path.exists(MASTER_FILE):
        print(f"Error: {MASTER_FILE} not found")
        return

    with open(MASTER_FILE, "r", encoding="utf-8") as f:
        master_content = f.read()
    master_keys = parse_tgbot_messages(master_content)

    print(f"Loaded {len(master_keys)} keys from MASTER")

    for filename in os.listdir(TRANS_DIR):
        if filename.endswith(".toml") and filename not in EXCLUDE_FILES:
            print(f"Processing {filename}...")
            process_file(os.path.join(TRANS_DIR, filename), master_keys)

    print("Done.")


if __name__ == "__main__":
    main()
