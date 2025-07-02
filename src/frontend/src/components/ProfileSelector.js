import React, { useState, useEffect, useCallback } from "react";
import {
  Select,
  MenuItem,
  Dialog,
  DialogTitle,
  DialogContent,
  TextField,
  Box,
  IconButton,
  Button,
  ListItemText,
} from "@mui/material";
import AddIcon from '@mui/icons-material/Add';
import EditIcon from '@mui/icons-material/Edit';
import DeleteIcon from '@mui/icons-material/Delete';
import { apiService } from "../services/apiService";
import { updateProfileId } from "../config";

const ProfileSelector = () => {
  const [profiles, setProfiles] = useState([]);
  const [selectedProfile, setSelectedProfile] = useState("");
  const [openDialog, setOpenDialog] = useState(false);
  const [openEditDialog, setOpenEditDialog] = useState(false);
  const [openDeleteDialog, setOpenDeleteDialog] = useState(false);
  const [newProfileName, setNewProfileName] = useState("");
  const [editingProfile, setEditingProfile] = useState(null);
  const [profileToDelete, setProfileToDelete] = useState(null);

  const handleProfileChange = useCallback(async (profileId) => {
    try {
      
      if (!profileId) {
        console.error("Invalid profileId received:", profileId);
        return;
      }

      await apiService.makeRequest("POST", "/profiles/active", {
        profile_id: profileId
      });
      
      setSelectedProfile(profileId);
      updateProfileId(profileId);
      window.dispatchEvent(new CustomEvent('profileChanged'));
    } catch (error) {
      console.error("Error setting active profile:", error);
    }
  }, []);

  const fetchProfiles = useCallback(async () => {
    try {
      const response = await apiService.makeRequest("GET", "/profiles");
      const profilesData = response.profiles || [];
      setProfiles(profilesData);
      
      const activeProfileResponse = await apiService.getActiveProfile();
      if (profilesData.length === 0) {
        try {
          const defaultProfile = await apiService.makeRequest("POST", "/profiles/single", {
            name: "Default Profile"
          });
          await apiService.makeRequest("POST", "/profiles/active", {
            profile_id: defaultProfile.profileID
          });
          await fetchProfiles();
        } catch (error) {
          console.error("Error creating default profile:", error);
        }
        return;
      }

      if (activeProfileResponse && activeProfileResponse.active) {
        setSelectedProfile(activeProfileResponse.profile_id);
      } else {
        handleProfileChange(profilesData[0].id);
      }
    } catch (error) {
      console.error("Error fetching profiles:", error);
    }
  }, [handleProfileChange]);

  useEffect(() => {
    fetchProfiles();
  }, [fetchProfiles]);

  const handleAddProfile = async () => {
    if (!newProfileName.trim()) return;

    try {
      await apiService.makeRequest("POST", "/profiles", {
        name: newProfileName.trim()
      });
      setNewProfileName("");
      setOpenDialog(false);
      await fetchProfiles();
    } catch (error) {
      console.error("Error adding profile:", error);
    }
  };

  const handleEditProfile = async () => {
    if (!newProfileName.trim() || !editingProfile) return;

    try {
      await apiService.makeRequest("PUT", `/profiles/single/${editingProfile.id}`, {
        name: newProfileName.trim()
      });
      setNewProfileName("");
      setOpenEditDialog(false);
      setEditingProfile(null);
      await fetchProfiles();
    } catch (error) {
      console.error("Error editing profile:", error);
    }
  };

  const handleDeleteProfile = async (profileId) => {
    if (profiles.length <= 1) {
      // Verhindere das Löschen des letzten Profils
      return;
    }
    setProfileToDelete(profileId);
    setOpenDeleteDialog(true);
  };

  const confirmDeleteProfile = async () => {
    try {
      await apiService.makeRequest("DELETE", `/profiles/single`, [], [profileToDelete]);
      await fetchProfiles();
      
      // Wenn das aktive Profil gelöscht wurde, wähle ein anderes aus
      if (selectedProfile === profileToDelete) {
        const remainingProfiles = profiles.filter(p => p.id !== profileToDelete);
        if (remainingProfiles.length > 0) {
          handleProfileChange(remainingProfiles[0].id);
        }
      }
      setOpenDeleteDialog(false);
      setProfileToDelete(null);
    } catch (error) {
      console.error("Error deleting profile:", error);
    }
  };

  const openEditDialogForProfile = (profile, event) => {
    event.stopPropagation();
    setEditingProfile(profile);
    setNewProfileName(profile.name);
    setOpenEditDialog(true);
  };

  return (
    <>
      <Box sx={{ display: 'flex', alignItems: 'center', gap: 1 }}>
        <Select
          value={selectedProfile || ""}
          onChange={(e) => handleProfileChange(e.target.value)}
          sx={{
            minWidth: 120,
            '& .MuiOutlinedInput-notchedOutline': {
              border: 'none'
            },
            '&:hover .MuiOutlinedInput-notchedOutline': {
              border: 'none'
            },
            '&.Mui-focused .MuiOutlinedInput-notchedOutline': {
              border: 'none'
            }
          }}
          renderValue={(value) => {
            const profile = profiles.find(p => p.id === value);
            return profile ? profile.name : "";
          }}
        >
          {Array.isArray(profiles) &&
            profiles.map((profile) => (
              <MenuItem key={profile.id} value={profile.id}>
                <Box sx={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', width: '100%' }}>
                  <ListItemText primary={profile.name} />
                  <Box sx={{ display: 'flex', ml: 1 }}>
                    <IconButton
                      size="small"
                      onClick={(e) => openEditDialogForProfile(profile, e)}
                      sx={{ mr: 1 }}
                    >
                      <EditIcon fontSize="small" />
                    </IconButton>
                    <IconButton
                      size="small"
                      onClick={(e) => {
                        e.stopPropagation();
                        handleDeleteProfile(profile.id);
                      }}
                      disabled={profiles.length <= 1}
                    >
                      <DeleteIcon fontSize="small" />
                    </IconButton>
                  </Box>
                </Box>
              </MenuItem>
            ))}
        </Select>
        <IconButton 
          onClick={() => setOpenDialog(true)}
          size="small"
          sx={{ 
            bgcolor: 'background.paper',
            '&:hover': { bgcolor: 'action.hover' }
          }}
        >
          <AddIcon />
        </IconButton>
      </Box>

      {/* Dialog für neues Profil */}
      <Dialog open={openDialog} onClose={() => setOpenDialog(false)}>
        <DialogTitle>Add New Profile</DialogTitle>
        <DialogContent>
          <TextField
            autoFocus
            margin="dense"
            label="Profile Name"
            type="text"
            fullWidth
            value={newProfileName}
            onChange={(e) => setNewProfileName(e.target.value)}
          />
        </DialogContent>
        <Box sx={{ p: 2, display: 'flex', justifyContent: 'flex-end' }}>
          <Button onClick={() => setOpenDialog(false)} sx={{ mr: 1 }}>
            Cancel
          </Button>
          <Button onClick={handleAddProfile} variant="contained" color="primary">
            Add
          </Button>
        </Box>
      </Dialog>

      {/* Dialog für Profilbearbeitung */}
      <Dialog open={openEditDialog} onClose={() => setOpenEditDialog(false)}>
        <DialogTitle>Edit Profile</DialogTitle>
        <DialogContent>
          <TextField
            autoFocus
            margin="dense"
            label="Profile Name"
            type="text"
            fullWidth
            value={newProfileName}
            onChange={(e) => setNewProfileName(e.target.value)}
          />
        </DialogContent>
        <Box sx={{ p: 2, display: 'flex', justifyContent: 'flex-end' }}>
          <Button onClick={() => setOpenEditDialog(false)} sx={{ mr: 1 }}>
            Cancel
          </Button>
          <Button onClick={handleEditProfile} variant="contained" color="primary">
            Save
          </Button>
        </Box>
      </Dialog>

      {/* Dialog für Profillöschung */}
      <Dialog open={openDeleteDialog} onClose={() => setOpenDeleteDialog(false)}>
        <DialogTitle>Delete Profile</DialogTitle>
        <DialogContent>
          <Box sx={{ p: 2 }}>
            <p>Do you really want to delete the profile "{profileToDelete?.name}"?</p>
            <Box sx={{ display: 'flex', justifyContent: 'flex-end', gap: 1, mt: 2 }}>
              <Button onClick={() => setOpenDeleteDialog(false)}>No</Button>
              <Button onClick={confirmDeleteProfile} color="error" variant="contained">
                Yes
              </Button>
            </Box>
          </Box>
        </DialogContent>
      </Dialog>
    </>
  );
};

export default ProfileSelector;
