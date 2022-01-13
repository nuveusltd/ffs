# ffs

I have a lot of folders that is stored different cloud drives like Dropbox, GoogleDrive.   
And I have some storage space on them.  

I thought, can I use these spaces like a physical drives. If I can build a [cgofuse](https://github.com/billziss-gh/cgofuse) based system like Raid5 and encrypt the parts, I will have got sorted single filesystem,redundancy and security.  
For Example, DropBox is PartA (encrypted), Google is PartB(encrypted) and ICloud is the checksum store. 

If Dropbox fails we can live with Google & iCloud or If Google fails, we can live with DropBox & iCloud. Also, All files securely stored.
