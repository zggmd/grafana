import React, { useState } from 'react';

import { Space } from '@grafana/experimental';
import { config } from '@grafana/runtime';
import { Alert, Button, Field, Modal } from '@grafana/ui';
import { P } from '@grafana/ui/src/unstable';
import { NestedFolderPicker } from 'app/core/components/NestedFolderPicker/NestedFolderPicker';
import { FolderChange, ROOT_FOLDER } from 'app/core/components/NestedFolderPicker/types';
import { FolderPicker } from 'app/core/components/Select/FolderPicker';

import { DashboardTreeSelection } from '../../types';

import { DescendantCount } from './DescendantCount';

export interface Props {
  isOpen: boolean;
  onConfirm: (targetFolderUid: string) => Promise<void>;
  onDismiss: () => void;
  selectedItems: DashboardTreeSelection;
}

export const MoveModal = ({ onConfirm, onDismiss, selectedItems, ...props }: Props) => {
  const [moveTarget, setMoveTarget] = useState<string>();
  const [isMoving, setIsMoving] = useState(false);
  const selectedFolders = Object.keys(selectedItems.folder).filter((uid) => selectedItems.folder[uid]);

  const handleFolderChange = (newFolder: FolderChange) => {
    setMoveTarget(newFolder.uid === ROOT_FOLDER ? '' : newFolder.uid);
  };

  const onMove = async () => {
    if (moveTarget !== undefined) {
      setIsMoving(true);
      try {
        await onConfirm(moveTarget);
        setIsMoving(false);
        onDismiss();
      } catch {
        setIsMoving(false);
      }
    }
  };

  return (
    <Modal title="Move" onDismiss={onDismiss} {...props}>
      {selectedFolders.length > 0 && <Alert severity="info" title="Moving this item may change its permissions." />}

      <P>This action will move the following content:</P>

      <DescendantCount selectedItems={selectedItems} />

      <Space v={3} />

      <Field label="Folder name">
        {config.featureToggles.nestedFolderPicker ? (
          <NestedFolderPicker value={moveTarget} onChange={handleFolderChange} />
        ) : (
          <FolderPicker allowEmpty onChange={handleFolderChange} />
        )}
      </Field>

      <Modal.ButtonRow>
        <Button onClick={onDismiss} variant="secondary" fill="outline">
          Cancel
        </Button>
        <Button disabled={moveTarget === undefined || isMoving} onClick={onMove} variant="primary">
          {isMoving ? 'Moving...' : 'Move'}
        </Button>
      </Modal.ButtonRow>
    </Modal>
  );
};
