package main

func Process() error {

	// if !strings.HasSuffix(filename, "_duplex-odd.pdf") {
	// 	go func() {

	// 		var (
	// 			sourcefile string
	// 			err        error
	// 		)

	// 		if strings.HasSuffix(filename, "_duplex-even.pdf") {
	// 			sourcefile, err = TryJoinPages(d.targetdir, filename)
	// 			if err != nil {
	// 				log.Printf("de-duplex pages: %v", err)
	// 			}
	// 		} else {
	// 			sourcefile = filepath.Join(d.targetdir, filename)
	// 		}

	// 		log.Printf("running post-process for %v in the background", sourcefile)

	// 		processed, err := PostProcess(sourcefile)
	// 		if err != nil {
	// 			log.Printf("post-processing %v failed: %v", sourcefile, err)
	// 		} else {
	// 			log.Printf("successfully ran post-process on %v", sourcefile)

	// 			err = os.Rename(processed, sourcefile)
	// 			if err != nil {
	// 				log.Printf("renaming %v failed: %v", sourcefile, err)
	// 			}
	// 		}

	// 		// store copy for consumption by paperless
	// 		if d.copydir != "" {
	// 			filename := time.Now().UTC().Format("20060102150405Z") + ".pdf"

	// 			err = copyFile(sourcefile, filepath.Join(d.copydir, filename))
	// 			if err != nil {
	// 				log.Printf("error storing copy in paperless incoming dir %v: %v", d.copydir, err)
	// 			} else {
	// 				log.Printf("stored copy as %v in paperless incoming dir", filename)
	// 			}
	// 		}
	// 	}()
	// }

	return nil
}
