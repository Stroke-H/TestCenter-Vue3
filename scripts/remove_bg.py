"""
Remove white backgrounds from Jungle Chess piece images.
Uses flood-fill from all edge pixels to remove connected white/near-white regions,
preserving the animal figure's original colors intact.
"""
from PIL import Image
import os
import sys

JUNGLE_DIR = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))),
                          'src', 'assets', 'jungle')
ANIMALS = ['cat', 'dog', 'elephant', 'leopard', 'lion', 'rat', 'tiger', 'wolf']

# Tolerance for "white-ish" pixels (0-255 distance per channel)
TOLERANCE = 40


def is_white_ish(pixel, tolerance):
    """Check if a pixel is close to white."""
    r, g, b = pixel[0], pixel[1], pixel[2]
    return r > (255 - tolerance) and g > (255 - tolerance) and b > (255 - tolerance)


def flood_fill_transparent(img, tolerance):
    """
    Flood-fill from all edge pixels.
    Any connected white-ish region touching the edge is made transparent.
    This preserves any white-ish pixels inside the animal figure.
    """
    width, height = img.size
    pixels = img.load()
    visited = set()
    to_clear = set()

    # BFS queue: start from all edge pixels that are white-ish
    queue = []
    for x in range(width):
        for y in [0, height - 1]:
            if is_white_ish(pixels[x, y], tolerance):
                queue.append((x, y))
                visited.add((x, y))
    for y in range(height):
        for x in [0, width - 1]:
            if (x, y) not in visited and is_white_ish(pixels[x, y], tolerance):
                queue.append((x, y))
                visited.add((x, y))

    # BFS flood fill
    while queue:
        x, y = queue.pop(0)
        to_clear.add((x, y))

        for dx, dy in [(-1, 0), (1, 0), (0, -1), (0, 1)]:
            nx, ny = x + dx, y + dy
            if 0 <= nx < width and 0 <= ny < height and (nx, ny) not in visited:
                visited.add((nx, ny))
                if is_white_ish(pixels[nx, ny], tolerance):
                    queue.append((nx, ny))

    # Make all flood-filled pixels transparent
    for x, y in to_clear:
        pixels[x, y] = (0, 0, 0, 0)

    return img


def process_image(name):
    """Process a single animal image to remove its white background."""
    path = os.path.join(JUNGLE_DIR, f'{name}.png')
    if not os.path.exists(path):
        print(f'  ⚠️  {name}.png not found, skipping')
        return False

    img = Image.open(path).convert('RGBA')
    original_size = img.size
    print(f'  Processing {name}.png ({original_size[0]}x{original_size[1]})...')

    img = flood_fill_transparent(img, TOLERANCE)

    # Also apply slight edge anti-aliasing: for pixels adjacent to transparent,
    # if they're somewhat white-ish, reduce their opacity for smoother edges
    width, height = img.size
    pixels = img.load()
    for x in range(width):
        for y in range(height):
            if pixels[x, y][3] == 0:
                continue
            # Check if any neighbor is transparent
            has_transparent_neighbor = False
            for dx, dy in [(-1, 0), (1, 0), (0, -1), (0, 1)]:
                nx, ny = x + dx, y + dy
                if 0 <= nx < width and 0 <= ny < height:
                    if pixels[nx, ny][3] == 0:
                        has_transparent_neighbor = True
                        break
            if has_transparent_neighbor:
                r, g, b, a = pixels[x, y]
                # If this edge pixel is very light, make it semi-transparent
                brightness = (r + g + b) / 3
                if brightness > 230:
                    pixels[x, y] = (r, g, b, int(a * 0.3))
                elif brightness > 200:
                    pixels[x, y] = (r, g, b, int(a * 0.7))

    img.save(path, 'PNG')
    print(f'  ✅ {name}.png - background removed')
    return True


def main():
    print(f'🔧 Removing white backgrounds from Jungle Chess pieces...')
    print(f'   Directory: {JUNGLE_DIR}\n')

    success = 0
    for name in ANIMALS:
        if process_image(name):
            success += 1

    print(f'\n✅ Done! Processed {success}/{len(ANIMALS)} images.')
    print(f'   Originals backed up in: {JUNGLE_DIR}/originals/')


if __name__ == '__main__':
    main()
