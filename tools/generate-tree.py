
#!/usr/bin/env python3
"""
Script to generate tree.json for LeafWiki from markdown files.
Scans data/root directory and creates the appropriate tree structure.
Automatically renames files and folders to follow LeafWiki conventions (lowercase, hyphens).
"""

import os
import json
import random
import string
import re


def generate_id(length=9):
    """Generate a random ID similar to the LeafWiki format."""
    chars = string.ascii_letters + string.digits + '_-'
    return ''.join(random.choice(chars) for _ in range(length))


def slug_to_title(slug):
    """Convert a slug to a title by capitalizing words."""
    # Replace hyphens and underscores with spaces
    title = slug.replace('-', ' ').replace('_', ' ')
    
    # Handle camelCase by inserting spaces before capitals
    title = re.sub(r'([a-z])([A-Z])', r'\1 \2', title)
    
    # Capitalize each word
    title = ' '.join(word.capitalize() for word in title.split())
    
    return title


def normalize_name(name):
    """
    Normalize a filename/foldername to LeafWiki conventions.
    - Lowercase
    - Replace underscores with hyphens
    
    Args:
        name: Original name
    
    Returns:
        str: Normalized name
    """
    return name.lower().replace('_', '-')


def rename_to_leafwiki_convention(path, dry_run=False):
    """
    Rename a file or folder to follow LeafWiki conventions if needed.
    
    Args:
        path: Path to the file or folder
        dry_run: If True, only print what would be renamed without doing it
    
    Returns:
        str: The new path (or original if no rename needed)
    """
    directory = os.path.dirname(path)
    basename = os.path.basename(path)
    
    # For files, keep the extension separate
    if os.path.isfile(path) and basename.endswith('.md'):
        name_part = basename[:-3]
        normalized = normalize_name(name_part)
        new_basename = normalized + '.md'
    else:
        new_basename = normalize_name(basename)
    
    # Check if rename is needed
    if basename == new_basename:
        return path
    
    new_path = os.path.join(directory, new_basename)
    
    if dry_run:
        print(f"Would rename: {basename} -> {new_basename}")
        return new_path
    
    # Perform the rename
    try:
        os.rename(path, new_path)
        print(f"Renamed: {basename} -> {new_basename}")
        return new_path
    except OSError as e:
        print(f"Error renaming {path}: {e}")
        return path


def process_directory(path, parent_slug='root', dry_run=False):
    """
    Process a directory and return its node structure.
    Automatically creates blank index.md files for folders that lack them.
    Renames files and folders to follow leafwiki conventions.
    
    Args:
        path: Path to the directory to process
        parent_slug: Slug of the parent (for generating proper slugs)
        dry_run: If True, don't actually rename files or create index.md files
    
    Returns:
        list: List of child nodes
    """
    children = []
    position = 0
    
    # Get all items in the directory
    try:
        items = sorted(os.listdir(path))
    except OSError as e:
        print(f"Error reading directory {path}: {e}")
        return []
    
    # First pass: rename files and directories to follow leafwiki conventions
    renamed_items = []
    for item in items:
        item_path = os.path.join(path, item)
        if item != 'index.md':  # Don't rename index.md
            new_path = rename_to_leafwiki_convention(item_path, dry_run)
            renamed_items.append(os.path.basename(new_path))
        else:
            renamed_items.append(item)
    
    # Re-sort after renaming
    items = sorted(renamed_items)
    
    # Separate files and directories
    md_files = []
    directories = []
    
    for item in items:
        item_path = os.path.join(path, item)
        if os.path.isfile(item_path) and item.endswith('.md') and item != 'index.md':
            md_files.append(item)
        elif os.path.isdir(item_path):
            directories.append(item)
    
    # Get list of directory names for checking duplicates
    dir_names = set(directories)
    
    # Process markdown files (excluding index.md)
    # Skip files that have a corresponding directory with the same name
    for md_file in md_files:
        file_base = md_file[:-3]  # Remove .md extension
        # Slug is already normalized from the rename
        slug = file_base
        
        # Skip this file if a directory with the same name exists
        # (the directory's index.md will represent this page instead)
        if file_base in dir_names:
            print(f"Skipping {md_file} - using folder {file_base}/ instead")
            continue
        
        node = {
            'id': generate_id(),
            'title': slug_to_title(file_base),
            'slug': slug,
            'children': [],
            'position': position
        }
        children.append(node)
        position += 1
    
    # Process subdirectories
    for directory in directories:
        dir_path = os.path.join(path, directory)
        # Slug is already normalized from the rename
        slug = directory
        index_path = os.path.join(dir_path, 'index.md')
        
        # Check if directory has an index.md, create if missing
        if not os.path.exists(index_path):
            if dry_run:
                print(f"Would create index.md for: {dir_path}")
            else:
                # Create a blank index.md file
                try:
                    with open(index_path, 'w', encoding='utf-8') as f:
                        f.write(f"# {slug_to_title(directory)}\n\n")
                    print(f"Created index.md for: {dir_path}")
                except OSError as e:
                    print(f"Error creating index.md in {dir_path}: {e}")
                    continue
        
        # Process the directory's children
        dir_children = process_directory(dir_path, slug, dry_run)
        
        node = {
            'id': generate_id(),
            'title': slug_to_title(directory),
            'slug': slug,
            'children': dir_children,
            'position': position
        }
        children.append(node)
        position += 1
    
    return children


def generate_tree_json(root_path='data/root', output_path='data/tree.json', dry_run=False):
    """
    Generate the tree.json file from markdown files.
    Automatically creates blank index.md files for folders that lack them.
    Automatically renames files and folders to follow LeafWiki conventions.
    
    Args:
        root_path: Path to the root directory containing markdown files
        output_path: Path where tree.json should be written
        dry_run: If True, preview changes without actually making them
    """
    if not os.path.exists(root_path):
        print(f"Error: Root path {root_path} does not exist!")
        return False
    
    # Check if root has index.md, create if missing
    root_index_path = os.path.join(root_path, 'index.md')
    if not os.path.exists(root_index_path):
        if dry_run:
            print(f"Would create index.md for root directory")
        else:
            try:
                with open(root_index_path, 'w', encoding='utf-8') as f:
                    f.write("# Home\n\n")
                print(f"Created index.md for root directory")
            except OSError as e:
                print(f"Error creating root index.md: {e}")
    
    # Process the root directory
    children = process_directory(root_path, dry_run=dry_run)
    
    # Create the root node
    tree = {
        'id': 'root',
        'title': 'root',
        'slug': 'root',
        'children': children,
        'position': 0
    }
    
    if dry_run:
        print("\n=== DRY RUN - No files were modified ===")
        print(f"Tree structure preview:")
        print_tree(tree)
        return True
    
    # Write to file
    try:
        with open(output_path, 'w', encoding='utf-8') as f:
            json.dump(tree, f, ensure_ascii=False, separators=(',', ':'))
        print(f"\nSuccessfully generated {output_path}")
        print(f"Total nodes: {count_nodes(tree)}")
        return True
    except OSError as e:
        print(f"Error writing to {output_path}: {e}")
        return False


def count_nodes(node):
    """Count total nodes in the tree."""
    count = 1
    for child in node.get('children', []):
        count += count_nodes(child)
    return count


def print_tree(node, indent=0):
    """Print the tree structure for verification."""
    print('  ' * indent + f"- {node['title']} ({node['slug']})")
    for child in node.get('children', []):
        print_tree(child, indent + 1)


if __name__ == '__main__':
    import argparse
    
    parser = argparse.ArgumentParser(
        description='Generate tree.json for LeafWiki from markdown files. Automatically renames files/folders to LeafWiki conventions (lowercase, hyphens).'
    )
    parser.add_argument(
        '--root',
        default='data/root',
        help='Path to root directory (default: data/root)'
    )
    parser.add_argument(
        '--output',
        default='data/tree.json',
        help='Output path for tree.json (default: data/tree.json)'
    )
    parser.add_argument(
        '--preview',
        action='store_true',
        help='Preview changes without actually modifying files or generating tree.json'
    )
    
    args = parser.parse_args()
    
    # Generate the tree
    generate_tree_json(args.root, args.output, dry_run=args.preview)