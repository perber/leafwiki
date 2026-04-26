/**
 * Drag-and-drop sort handler for sort dialogs.
 * Replaces arrow-based ordering with intuitive drag reordering.
 */

let dragSrcEl = null;

function handleDragStart(e) {
  dragSrcEl = this;
  e.dataTransfer.effectAllowed = 'move';
  e.dataTransfer.setData('text/html', this.innerHTML);
  this.classList.add('dragging');
}

function handleDragOver(e) {
  if (e.preventDefault) e.preventDefault();
  e.dataTransfer.dropEffect = 'move';
  return false;
}

function handleDragEnter(e) {
  this.classList.add('over');
}

function handleDragLeave(e) {
  this.classList.remove('over');
}

function handleDrop(e) {
  if (e.stopPropagation) e.stopPropagation();
  if (dragSrcEl !== this) {
    const list = this.parentNode;
    const items = Array.from(list.children);
    const srcIdx = items.indexOf(dragSrcEl);
    const dstIdx = items.indexOf(this);
    if (srcIdx < dstIdx) {
      this.after(dragSrcEl);
    } else {
      this.before(dragSrcEl);
    }
    list.dispatchEvent(new CustomEvent('sort-reorder', { detail: getSortOrder(list) }));
  }
  return false;
}

function handleDragEnd() {
  this.classList.remove('dragging');
  const items = document.querySelectorAll('.sortable-item');
  items.forEach(item => item.classList.remove('over'));
}

function getSortOrder(list) {
  return Array.from(list.children).map(item => item.dataset.id);
}

function makeSortable(container) {
  const items = container.querySelectorAll('.sortable-item');
  items.forEach(item => {
    item.setAttribute('draggable', 'true');
    item.addEventListener('dragstart', handleDragStart, false);
    item.addEventListener('dragenter', handleDragEnter, false);
    item.addEventListener('dragover', handleDragOver, false);
    item.addEventListener('dragleave', handleDragLeave, false);
    item.addEventListener('drop', handleDrop, false);
    item.addEventListener('dragend', handleDragEnd, false);
  });
}

export { makeSortable, getSortOrder };
